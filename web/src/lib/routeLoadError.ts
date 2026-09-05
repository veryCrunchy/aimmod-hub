export function isRouteLoadError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : "";
  return /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed|unable to preload css|loading (?:css )?chunk [\w-]+ failed/i.test(message);
}
