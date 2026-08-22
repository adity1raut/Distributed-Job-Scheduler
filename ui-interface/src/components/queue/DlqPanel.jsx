import { RotateCcw, ShieldCheck } from 'lucide-react'
import toast from 'react-hot-toast'
import { listDeadLetters, replayDeadLetter } from '../../api/dlq'
import { usePolling } from '../../hooks/usePolling'
import CopyableId from '../CopyableId'
import EmptyState from '../EmptyState'
import ErrorBanner from '../ErrorBanner'
import { SkeletonRows } from '../Skeleton'
import Timestamp from '../Timestamp'

export default function DlqPanel({ queueId }) {
  const { data, error, loading, reload } = usePolling(() => listDeadLetters(queueId), [queueId], 5000)

  const handleReplay = async (id) => {
    try {
      await replayDeadLetter(id)
      reload()
      toast.success('Job requeued for replay')
    } catch (err) {
      toast.error(err.message)
    }
  }

  return (
    <div>
      <ErrorBanner message={error} />
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Job</th>
              <th>Failed</th>
              <th>Error</th>
              <th>Replayed</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && !data && <SkeletonRows rows={3} cols={5} />}
            {data?.map((entry) => (
              <tr key={entry.id}>
                <td>
                  <CopyableId id={entry.job_id} to={`/jobs/${entry.job_id}`} />
                </td>
                <td>
                  <Timestamp value={entry.failed_at} />
                </td>
                <td className="wrap-cell error-cell">{entry.final_error}</td>
                <td>{entry.replayed ? 'yes' : 'no'}</td>
                <td>
                  <button className="btn-ghost" disabled={entry.replayed} onClick={() => handleReplay(entry.id)}>
                    <RotateCcw size={13} />
                    Replay
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data && data.length === 0 && (
        <EmptyState icon={ShieldCheck} title="Nothing dead-lettered" hint="Jobs land here once retries are exhausted." />
      )}
    </div>
  )
}
