import type { ReactNode } from "react";

// Proyeccion isometrica: x avanza hacia abajo-derecha, y hacia abajo-izquierda, z hacia arriba.
const C = 0.866;
const S = 0.5;
type Punto = [number, number];

function Losa({
  ox,
  oy,
  a,
  b,
  h,
  tapa,
  lado,
  frente,
  paginas,
  children,
}: {
  ox: number;
  oy: number;
  a: number;
  b: number;
  h: number;
  tapa: string;
  lado: string;
  frente: string;
  paginas?: string;
  children?: ReactNode;
}) {
  const P = (x: number, y: number, z: number): Punto => [ox + (x - y) * C, oy + (x + y) * S - z];
  const poly = (...pts: Punto[]) => pts.map((p) => p.join(",")).join(" ");
  const [tx, ty] = P(0, 0, h);
  return (
    <g>
      <polygon points={poly(P(0, b, h), P(a, b, h), P(a, b, 0), P(0, b, 0))} fill={lado} />
      <polygon points={poly(P(a, 0, h), P(a, b, h), P(a, b, 0), P(a, 0, 0))} fill={frente} />
      {paginas && <polygon points={poly(P(a, 0, h - 4), P(a, b, h - 4), P(a, b, 4), P(a, 0, 4))} fill={paginas} />}
      <g transform={`matrix(${C} ${S} ${-C} ${S} ${tx} ${ty})`}>
        <rect width={a} height={b} rx="5" fill={tapa} />
        {children}
      </g>
    </g>
  );
}

/** Pila de libros con una tableta encima, en isometrico: ilustracion de las pantallas de inicio. */
export function HeroArt() {
  return (
    <svg className="hero-art" viewBox="0 0 300 250" aria-hidden="true">
      <ellipse cx="160" cy="214" rx="120" ry="24" fill="#c9cfee" opacity=".55" />
      <Losa ox={140} oy={118} a={150} b={104} h={24} tapa="#4a5ce6" lado="#3544b8" frente="#5b6df0" paginas="#f4f5fc" />
      <Losa ox={148} oy={94} a={140} b={98} h={22} tapa="#f7a5b3" lado="#e98a9b" frente="#f4a0ae" paginas="#fff7f9" />
      <Losa ox={152} oy={48} a={112} b={78} h={6} tapa="#5b67d6" lado="#4552b4" frente="#6874de">
        <rect x="5" y="5" width="102" height="68" rx="4" fill="#eceffc" />
        {[18, 32, 46, 60].map((x) => (
          <circle key={x} cx={x} cy="17" r="3.4" fill="#4a5ce6" />
        ))}
        <rect x="16" y="28" width="74" height="9" rx="3" fill="#cbd1ef" />
        <rect x="16" y="43" width="56" height="9" rx="3" fill="#cbd1ef" />
        <rect x="16" y="58" width="66" height="6" rx="3" fill="#dde1f6" />
      </Losa>
    </svg>
  );
}
