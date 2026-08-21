import toast from 'react-hot-toast'
import { listDeadLetters, replayDeadLetter } from '../../api/dlq'
import { usePolling } from '../../hooks/usePolling'
import ErrorBanner from '../ErrorBanner'

export default function DlqPanel({ queueId }) {
  const { data, error, reload } = usePolling(() => listDeadLetters(queueId), [queueId], 5000)

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
              <th>Failed at</th>
              <th>Error</th>
              <th>Replayed</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {data?.map((entry) => (
              <tr key={entry.id}>
                <td className="mono">{entry.job_id.slice(0, 8)}</td>
                <td>{new Date(entry.failed_at).toLocaleString()}</td>
                <td className="wrap-cell">{entry.final_error}</td>
                <td>{entry.replayed ? 'yes' : 'no'}</td>
                <td>
                  <button className="btn-ghost" disabled={entry.replayed} onClick={() => handleReplay(entry.id)}>
                    Replay
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {data && data.length === 0 && <p className="muted">Nothing dead-lettered on this queue.</p>}
    </div>
  )
}
