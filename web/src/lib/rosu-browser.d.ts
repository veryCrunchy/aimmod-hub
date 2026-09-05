declare module "virtual:rosu-browser" {
  export function loadRosu(): Promise<typeof import("rosu-pp-js")>;
}
