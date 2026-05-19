export function SummaryStrip({ counts }: { counts: Record<string, number> }) {
  return (
    <section className="summary" aria-label="Resource summary">
      <div>
        <span>{counts.projects}</span>
        <p>Projects</p>
      </div>
      <div>
        <span>{counts.templates}</span>
        <p>Templates</p>
      </div>
      <div>
        <span>{counts.sandboxes}</span>
        <p>Sandboxes</p>
      </div>
      <div>
        <span>{counts.running}</span>
        <p>Running</p>
      </div>
    </section>
  )
}
