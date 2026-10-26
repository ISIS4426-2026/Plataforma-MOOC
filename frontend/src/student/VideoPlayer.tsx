import { useEffect, useRef, useState } from "react";
import { hlsManifestUrl } from "../runtime";

interface Props {
  resourceStableId: string;
  /** Segundos de reproduccion acumulados desde el ultimo latido (se informa cada ~15 s y al terminar). */
  onBeat: (dwellSeconds: number, completed: boolean) => void;
}

/**
 * Reproduce el video desde el almacenamiento de objetos (el manifiesto HLS), NO desde la API:
 * los segmentos nunca pasan por las instancias de API. hls.js se carga solo cuando se abre un
 * video (division de codigo), y Safari usa su reproductor nativo.
 */
export function VideoPlayer({ resourceStableId, onBeat }: Props) {
  const ref = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState<string | null>(null);
  const url = hlsManifestUrl(resourceStableId);
  // onBeat cambia en cada render del padre: se guarda en una referencia para no reiniciar el reproductor.
  const beat = useRef(onBeat);
  beat.current = onBeat;

  useEffect(() => {
    const video = ref.current;
    if (!video || !url) return;
    let hls: { destroy: () => void } | null = null;
    let cancelled = false;
    setError(null);

    (async () => {
      if (video.canPlayType("application/vnd.apple.mpegurl")) {
        video.src = url;
        return;
      }
      const { default: Hls } = await import("hls.js");
      if (cancelled) return;
      if (!Hls.isSupported()) {
        setError("Tu navegador no puede reproducir este video.");
        return;
      }
      const instancia = new Hls();
      hls = instancia;
      instancia.on(Hls.Events.ERROR, (_e, data) => {
        if (data.fatal) setError("El video todavía no está disponible o no se pudo cargar.");
      });
      instancia.loadSource(url);
      instancia.attachMedia(video);
    })().catch(() => setError("No se pudo iniciar el reproductor."));

    // Latido cada 15 s mientras se reproduce, para que el servidor registre la permanencia.
    let acumulado = 0;
    const reloj = window.setInterval(() => {
      if (!video.paused && !video.ended) {
        acumulado += 15;
        beat.current(acumulado, false);
        acumulado = 0;
      }
    }, 15000);
    const alTerminar = () => {
      beat.current(acumulado, true);
      acumulado = 0;
    };
    video.addEventListener("ended", alTerminar);
    // Reproduccion nativa (Safari, Chrome reciente): el error llega al elemento, no a hls.js.
    const alFallar = () => setError("El video todavía no está disponible o no se pudo cargar.");
    video.addEventListener("error", alFallar);

    return () => {
      cancelled = true;
      window.clearInterval(reloj);
      video.removeEventListener("ended", alTerminar);
      video.removeEventListener("error", alFallar);
      hls?.destroy();
    };
  }, [url, resourceStableId]);

  if (!url) {
    return (
      <p role="alert" className="error">
        No hay un origen de video configurado para este entorno.
      </p>
    );
  }
  return (
    <div>
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video ref={ref} controls preload="metadata" className="video" aria-label="Reproductor de video" />
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
    </div>
  );
}
