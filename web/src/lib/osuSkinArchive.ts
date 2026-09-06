import { unzip, zipSync } from "fflate";

const gameplay = /^(hitcircle|hitcircleoverlay|approachcircle|slider[b0-9a-z-]*|reversearrow|followpoint(-[0-9]+)?|cursor[a-z]*|hit(0|50|100|300)[a-z0-9-]*|spinner-[a-z0-9-]+|inputoverlay-[a-z0-9-]+|selection-mod-[a-z0-9-]+)(@2x)?\.png$/;
const sound = /^((normal|soft|drum)-(hit(normal|whistle|finish|clap)|slider(slide|tick|whistle))|combobreak|spinnerspin|spinnerbonus)\.(wav|ogg|mp3)$/;
const normalize = (name: string) => name.replaceAll("\\", "/").toLowerCase();
const safe = (name: string) => !name.startsWith("/") && !name.includes(":") && !name.split("/").some(part => part === ".." || part === ".");

function unpack(bytes: Uint8Array, accepts: (name: string) => boolean): Promise<Record<string, Uint8Array>> {
  return new Promise((resolve, reject) => {
    let total = 0;
    let count = 0;
    const names = new Set<string>();
    try {
      unzip(bytes, { filter: file => {
        if (++count > 4096) throw new Error("This skin contains too many files.");
        const name = normalize(file.name);
        if (!safe(name) || !accepts(name)) return false;
        if (names.has(name)) throw new Error("This skin contains duplicate assets.");
        names.add(name);
        total += file.originalSize;
        if (file.originalSize > 8 * 1024 * 1024 || total > 32 * 1024 * 1024) throw new Error("This skin exceeds the gameplay asset size limit.");
        return true;
      } }, (error, files) => error ? reject(error) : resolve(Object.fromEntries(Object.entries(files).map(([name, data]) => [normalize(name), data]))));
    } catch (error) { reject(error); }
  });
}

/** Filter before decoding images/audio, including custom font paths from skin.ini. */
export async function prepareSkinArchive(buffer: ArrayBuffer): Promise<ArrayBuffer> {
  if (buffer.byteLength > 64 * 1024 * 1024) throw new Error("Choose a skin smaller than 64 MB.");
  const bytes = new Uint8Array(buffer);
  const settings = await unpack(bytes, name => name === "skin.ini" || name.endsWith("/skin.ini"));
  const iniPath = settings["skin.ini"] ? "skin.ini" : Object.keys(settings).length === 1 ? Object.keys(settings)[0] : undefined;
  if (!iniPath || settings[iniPath].length > 64 * 1024) throw new Error("Choose an osu! skin with a skin.ini file.");
  const root = iniPath.slice(0, -"skin.ini".length);
  const ini = new TextDecoder().decode(settings[iniPath]);
  const prefixes = ["default", "score", "combo", "scoreentry"];
  for (const match of ini.matchAll(/^\s*(HitCirclePrefix|ScorePrefix|ComboPrefix)\s*:\s*([^\r\n]+)/gmi)) prefixes.push(normalize(match[2].split("//")[0].trim()));
  const files = await unpack(bytes, name => {
    if (!name.startsWith(root)) return false;
    name = name.slice(root.length);
    return name === "skin.ini" || gameplay.test(name) || sound.test(name) || prefixes.some(prefix => name.startsWith(`${prefix}-`) && name.endsWith(".png"));
  });
  const selected: Record<string, Uint8Array> = {};
  let pixels = 0;
  for (const [path, data] of Object.entries(files)) {
    const name = path.slice(root.length);
    if (name.endsWith(".png")) {
      // Bound decoded bitmap memory before createImageBitmap allocates it.
      if (data.length < 24 || data[0] !== 137 || data[1] !== 80 || data[2] !== 78 || data[3] !== 71) throw new Error("This skin contains an invalid image.");
      const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
      const width = view.getUint32(16), height = view.getUint32(20);
      pixels += width * height;
      if (!width || !height || width > 4096 || height > 4096 || pixels > 32 * 1024 * 1024) throw new Error("This skin's images exceed the playback size limit.");
    }
    selected[name] = data;
  }
  if (!Object.keys(selected).some(name => /(^hitcircle|^cursor).*\.png$/.test(name))) throw new Error("This skin does not contain osu! gameplay artwork.");
  return Uint8Array.from(zipSync(selected, { level: 0 })).buffer;
}
