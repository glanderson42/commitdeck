import { Clock } from "lucide-react";
import type { ActivityEvent } from "../../types";
import { timeAgo } from "../../components/ui";
export function ActivityTimeline({ events }: { events: ActivityEvent[] }) {
  return (
    <section className="activity-timeline">
      <h3>
        <Clock size={15} />
        Recent activity
      </h3>
      {!events.length && (
        <p className="muted">Meaningful workspace events will appear here.</p>
      )}
      <ol>
        {events.map((e) => (
          <li key={e.id}>
            <i
              className={`dot ${e.type.includes("failed") ? "red" : e.type === "permission_requested" ? "yellow" : e.type.includes("completed") || e.type === "ci_passed" ? "green" : "muted"}`}
            />
            <div>
              <span>{e.title}</span>
              <time dateTime={e.createdAt}>{timeAgo(e.createdAt)}</time>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}
