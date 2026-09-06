// Memory-only cache for public reads. Never use for account or private data.
export function createPublicQuery<T>(ttlMs = 0, now = Date.now) {
  const values = new Map<string, { value: T; expires: number }>();
  const pending = new Map<string, Promise<T>>();
  return (key: string, load: () => Promise<T>): Promise<T> => {
    const cached = values.get(key);
    if (cached && now() < cached.expires) return Promise.resolve(cached.value);
    const active = pending.get(key);
    if (active) return active;
    const request = Promise.resolve().then(load).then(value => {
      if (ttlMs > 0) {
        values.delete(key);
        if (values.size >= 64) values.delete(values.keys().next().value!);
        values.set(key, { value, expires: now() + ttlMs });
      }
      return value;
    }).finally(() => pending.delete(key));
    pending.set(key, request);
    return request;
  };
}
