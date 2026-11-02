// Tiempo al primer cuadro de UNA reproduccion HLS (G4, #224; criterios_de_medicion.md §6).
//
//   node ttff.js <https://.../master.m3u8>   -> una linea JSON por stdout
//
// Chromium sin ventana y hls.js. Mide, desde que se asigna el `master.m3u8` como fuente hasta el primer
// evento `playing`, y lo descompone en descarga del manifiesto, de la lista de variante, del segmento 0 y
// decodificacion inicial. La decodificacion inicial es `playing` menos la llegada del segmento 0: medida,
// no una constante (la Entrega 2 sumaba 45 ms fijos a una latencia HTTP y lo llamaba «reproductor real»).
const fs = require("fs");
const puppeteer = require("puppeteer-core");

(async () => {
  const url = process.argv[2];
  if (!url) { console.error("falta la URL del master.m3u8"); process.exit(2); }
  const hlsjs = fs.readFileSync(require.resolve("hls.js/dist/hls.min.js"), "utf8");
  const browser = await puppeteer.launch({
    executablePath: process.env.CHROMIUM || "/usr/bin/chromium",
    headless: "new",
    args: ["--no-sandbox", "--disable-gpu", "--mute-audio", "--autoplay-policy=no-user-gesture-required"],
  });
  let salida;
  try {
    const page = await browser.newPage();
    await page.setBypassCSP(true);
    await page.setContent("<!doctype html><video id=v muted playsinline></video>");
    await page.addScriptTag({ content: hlsjs });
    salida = await page.evaluate((src) => new Promise((resolve) => {
      const v = document.getElementById("v");
      const ev = {};
      let t0 = 0;
      const marca = (n) => { if (!(n in ev)) ev[n] = +(performance.now() - t0).toFixed(1); };
      ["loadstart", "loadedmetadata", "canplay", "playing", "waiting", "stalled"].forEach((n) => v.addEventListener(n, () => marca(n)));
      const hls = new Hls({ enableWorker: false });
      let primerSn = null;
      hls.on(Hls.Events.MANIFEST_LOADED, (e, d) => { ev.manifiesto_ms = +(d.stats.loading.end - d.stats.loading.start).toFixed(1); });
      hls.on(Hls.Events.LEVEL_LOADED, (e, d) => { if (!("variante_ms" in ev)) ev.variante_ms = +(d.stats.loading.end - d.stats.loading.start).toFixed(1); });
      hls.on(Hls.Events.FRAG_LOADED, (e, d) => {
        if (primerSn === null) primerSn = d.frag.sn;
        if (d.frag.sn === primerSn && !("segmento0_ms" in ev)) {
          ev.segmento0_ms = +(d.stats.loading.end - d.stats.loading.start).toFixed(1);
          ev.segmento0_llegada = +(d.stats.loading.end - t0).toFixed(1);
        }
      });
      hls.on(Hls.Events.ERROR, (e, d) => { if (d.fatal) { ev.error = d.type + ":" + d.details; hls.destroy(); resolve(ev); } });
      v.addEventListener("playing", () => { setTimeout(() => { hls.destroy(); resolve(ev); }, 300); });
      setTimeout(() => { ev.error = ev.error || "timeout 30s"; hls.destroy(); resolve(ev); }, 30000);
      t0 = performance.now();
      hls.loadSource(src);
      hls.attachMedia(v);
      v.play().catch(() => {});
    }), url);
    salida.navegador = await browser.version();
  } finally {
    await browser.close();
  }
  if (salida.playing != null) {
    salida.ttff_ms = salida.playing;
    if (salida.segmento0_llegada != null) salida.decodificacion_ms = +(salida.playing - salida.segmento0_llegada).toFixed(1);
  }
  console.log(JSON.stringify(salida));
})().catch((e) => { console.log(JSON.stringify({ error: String(e).slice(0, 200) })); process.exit(0); });
