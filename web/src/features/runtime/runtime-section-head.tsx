export function RuntimeSectionHead({ eyebrow, title }: { eyebrow: string; title: string }) {
  return (
    <div className="runtime-section-head">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h3>{title}</h3>
      </div>
    </div>
  )
}
