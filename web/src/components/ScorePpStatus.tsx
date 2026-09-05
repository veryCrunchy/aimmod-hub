export function ScorePpStatus({ pending, failed, retry }: { pending: number; failed: number; retry: () => void }) {
  if (!pending && !failed) return null;
  return <div className="my-3 flex flex-wrap items-center gap-3 text-sm" role="status">
    {pending > 0 && <span>Calculating PP for {pending} remaining plays...</span>}
    {failed > 0 && <><span>PP could not be calculated for {failed} plays.</span><button className="text-cyan underline" onClick={retry}>Retry calculations</button></>}
  </div>;
}
