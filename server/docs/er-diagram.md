# Entity-Relationship Diagram

Twelve tables, matching the brief exactly: Users, Organizations, Projects,
Queues, Jobs, Job Executions, Retry Policies, Workers, Worker Heartbeats,
Job Logs, Scheduled Jobs, Dead Letter Queue.

![Entity-relationship diagram of all twelve tables: organizations own users, projects, and retry policies; projects contain queues; queues hold jobs and define scheduled jobs which spawn more jobs; jobs produce job executions which emit job logs and can terminate in the dead letter queue; workers execute job executions and report worker heartbeats.](images/er-diagram.png)

## A few things worth knowing about this schema

Every table uses a randomly generated ID as its primary key instead of a
simple counter, mainly so IDs are safe to hand back in API responses
without leaking how many rows exist.

It's normalized throughout, and the clearest example is retry policies
being their own table instead of a few columns bolted onto queues. A
policy needs to be reusable across queues, and a single job occasionally
needs to override its queue's default. Neither of those work if the
policy is just inline columns. A job's policy reference is optional for
exactly this reason: unset, it falls back to whatever its queue's policy
says.

Job executions and job logs are append-only. A job's own row gets
overwritten every time it retries, but each *attempt* keeps its own row.
Otherwise there'd be no way to look back and see what actually happened
on attempt 2 once attempt 3 has already started.

### The one nullable exception

Every other relationship in this schema is required. A worker's
organization link is the one exception: it was added by a later
migration, and worker rows created before that migration have no
organization they can be correctly backfilled to. Deleting them instead
of leaving them unset would have taken their execution and log history
down with them. They simply stop matching any org-scoped query going
forward, equivalent to being retired, not deleted. Every worker row the
application creates going forward always sets it.

### The indexes that actually matter

Most foreign-key columns have an obvious supporting index and aren't
worth listing individually. A few carry real weight on the write path,
though:

- The index behind the job-claiming query, so claiming a job doesn't mean
  scanning the whole table.
- A uniqueness constraint on idempotency keys, so idempotent submission
  isn't just an application-layer promise: the database physically won't
  allow a duplicate.
- An index shaped around "is this worker still alive," since that's
  always a query for the single most recent heartbeat.
- A partial index on due, active scheduled jobs, since the scheduler only
  ever cares about those.

### What happens when something gets deleted

Most deletes cascade down the natural containment tree: organization →
project → queue → job → execution → log, plus dead-letter entries and
worker heartbeats cascading with their parent row. That's the right
default here: an execution log with no job to belong to isn't useful to
anyone. The trade-off is that deleting a project takes its whole audit
history with it, which is fine for this brief but flagged in
**[design-decisions.md](design-decisions.md)** as the first thing to change
(to a soft-delete) if this ever needed to hold onto records for
compliance reasons.

A few relationships deliberately don't follow that pattern: you can't
delete a retry policy that's still in use, and you can't delete a user
who still owns a project. And if a cron definition or a job's policy
override gets deleted later, a job that references it just loses that
reference rather than being deleted itself: a job that's already running
should keep running either way.
