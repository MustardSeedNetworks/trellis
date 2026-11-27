export interface SettingsFact {
  label: string;
  value: string;
}

/** A read-only label/value list: the shape About and Daemon share. */
export function SettingsFacts({ facts }: { facts: readonly SettingsFact[] }) {
  return (
    <dl className="flex flex-col divide-y divide-hairline">
      {facts.map((fact) => (
        <div key={fact.label} className="flex items-baseline justify-between gap-4 py-2">
          <dt className="kicker">{fact.label}</dt>
          <dd className="figure min-w-0 break-all text-right text-sm text-text-primary">
            {fact.value === '' ? '—' : fact.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
