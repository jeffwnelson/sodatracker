import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getMachineStatus } from "../api/client";
import type { Product } from "../api/types";
import { useMachine } from "../api/machine";
import { Card } from "../components/Card";
import { ProductDot } from "../components/ProductDot";

/*
 * The harness from the Pi's GPIO header, through the RJ45 breakout, to each
 * column's cam contact. Which GPIO pin is which slot comes from the slot
 * configuration; the physical side (header pin, wire color, breakout
 * position) is fixed here and in the README's Pin Assignments table, so keep
 * the two in step when the harness changes.
 *
 * The breakout and the cable's plug are both wired T568B, so position n is the
 * same conductor color as the plug's pin n. Cable colors are the Pi-side ones:
 * the cable's colors don't match end to end. A wire's `jumper` is set where the
 * Dupont jumper from the header to the breakout is a different color.
 *
 * PROPOSED is a planned rewire: the same breakout positions on header pins
 * 33–40, with jumpers in resistor color-code order. Once it's done, it becomes
 * CURRENT (and the slots' GPIO pins and piagent.yaml change to match).
 */

type Wire = {
  /** Breakout terminal, 1–8. */
  position: number;
  /** Physical header pin, 1–40. */
  physical: number;
  /** BCM number, or null for ground. */
  bcm: number | null;
  color: string;
  /** White-striped conductor (white/orange and so on). */
  striped: boolean;
  colorName: string;
  role: "signal" | "common" | "spare";
  /** The Dupont jumper from the header to the breakout, when its color
   * differs from the cable conductor's. */
  jumper?: Look;
};

/** How a conductor is drawn and named. */
type Look = { color: string; striped: boolean; name: string };

const cableLook = (w: Wire): Look => ({ color: w.color, striped: w.striped, name: w.colorName });
const jumperLook = (w: Wire): Look => w.jumper ?? cableLook(w);

const ORANGE = "#ea580c";
const GREEN = "#16a34a";
const BLUE = "#2563eb";
const BROWN = "#8b4513";
const PURPLE = "#7e22ce";
const BLACK = "#1c1917";
const WHITE = "#ffffff";
const DARK_GREY = "#525a66";
const YELLOW = "#facc15";
const RED = "#dc2626";
const GREY = "#6b7280";
/** Fills light enough to need dark text and a dark outline on a header pin. */
const LIGHT = new Set([WHITE, YELLOW]);

const CURRENT: Wire[] = [
  { position: 1, physical: 20, bcm: null, color: ORANGE, striped: true, colorName: "white/orange", role: "common",
    jumper: { color: PURPLE, striped: false, name: "purple" } },
  { position: 2, physical: 16, bcm: 23, color: ORANGE, striped: false, colorName: "orange", role: "spare",
    jumper: { color: BLACK, striped: false, name: "black" } },
  { position: 3, physical: 35, bcm: 19, color: GREEN, striped: true, colorName: "white/green", role: "signal",
    jumper: { color: WHITE, striped: false, name: "white" } },
  { position: 4, physical: 11, bcm: 17, color: BLUE, striped: false, colorName: "blue", role: "signal" },
  { position: 5, physical: 15, bcm: 22, color: BLUE, striped: true, colorName: "white/blue", role: "signal",
    jumper: { color: DARK_GREY, striped: false, name: "dark grey" } },
  { position: 6, physical: 33, bcm: 13, color: GREEN, striped: false, colorName: "green", role: "signal" },
  { position: 7, physical: 14, bcm: null, color: BROWN, striped: true, colorName: "white/brown", role: "common",
    jumper: { color: YELLOW, striped: false, name: "yellow" } },
  { position: 8, physical: 22, bcm: 25, color: BROWN, striped: false, colorName: "brown", role: "signal",
    jumper: { color: ORANGE, striped: false, name: "orange" } },
];

/** The rewire onto header pins 33–40, breakout position by position. */
const REWIRE: Record<number, { physical: number; bcm: number | null; jumper: Look }> = {
  1: { physical: 34, bcm: null, jumper: { color: BROWN, striped: false, name: "brown" } },
  2: { physical: 33, bcm: 13, jumper: { color: RED, striped: false, name: "red" } },
  3: { physical: 35, bcm: 19, jumper: { color: ORANGE, striped: false, name: "orange" } },
  4: { physical: 36, bcm: 16, jumper: { color: YELLOW, striped: false, name: "yellow" } },
  5: { physical: 37, bcm: 26, jumper: { color: GREEN, striped: false, name: "green" } },
  6: { physical: 38, bcm: 20, jumper: { color: BLUE, striped: false, name: "blue" } },
  7: { physical: 39, bcm: null, jumper: { color: PURPLE, striped: false, name: "purple" } },
  8: { physical: 40, bcm: 21, jumper: { color: GREY, striped: false, name: "grey" } },
};

const PROPOSED: Wire[] = CURRENT.map((w) => ({ ...w, ...REWIRE[w.position] }));

type Layout = "current" | "proposed";

/** Slots whose two Diet Mountain Dew columns may be the other way round. */
const UNCONFIRMED_SLOTS = new Set([1, 2]);

// --- Geometry (SVG user units) ---------------------------------------------

const W = 1000;
const H = 600;
const ROW = 26; // header row pitch
const HEADER_TOP = 70;
const ODD_X = 110;
const EVEN_X = 140;
const PIN_R = 10;
const EXIT_X = 190; // where the jumpers leave the header and start to curve
const LEAD_X = 320; // the straight, labelled lead into each breakout terminal
const BO_X = 430; // breakout box, left edge
const BO_W = 110;
const BO_TOP = 70;
const BO_PITCH = 62;
const JACK_W = 30;
/** Where conductors leave the jack toward the machine. */
const JACK_OUT = BO_X + BO_W - 8 + JACK_W;
const SW_X = 760; // contact, signal side
const SW_GAP = 34;
const BUS_X = SW_X + SW_GAP + 18;
const COL_TOP = 125;
const COL_PITCH = 92;

const pinY = (physical: number) => HEADER_TOP + (Math.ceil(physical / 2) - 1) * ROW;
const termY = (position: number) => BO_TOP + 30 + (position - 1) * BO_PITCH;
const colY = (index: number) => COL_TOP + index * COL_PITCH;

/**
 * A jumper's path out of the header. Odd pins sit in the inner column, so
 * their wire runs out between rows rather than over the even pin beside them.
 */
function headerExit(physical: number): { d: string; y: number } {
  const y = pinY(physical);
  if (physical % 2 === 0) return { d: `M ${EVEN_X} ${y} L ${EXIT_X} ${y}`, y };
  const gap = y - ROW / 2;
  return {
    d: `M ${ODD_X} ${y} L ${(ODD_X + EVEN_X) / 2} ${gap} L ${EXIT_X} ${gap}`,
    y: gap,
  };
}

/** A horizontal-ended S-curve from the current point (x1, y1) to (x2, y2). */
function curveTo(x1: number, y1: number, x2: number, y2: number) {
  const mx = (x1 + x2) / 2;
  return `C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`;
}

function curve(x1: number, y1: number, x2: number, y2: number) {
  return `M ${x1} ${y1} ${curveTo(x1, y1, x2, y2)}`;
}

/** A conductor: outline, color, and white stripes for the striped ones. */
function Conductor({ d, look, dim }: { d: string; look: Look; dim: boolean }) {
  return (
    <g opacity={dim ? 0.15 : 1} style={{ transition: "opacity 150ms" }}>
      <path d={d} fill="none" stroke="#44403c" strokeWidth={6} strokeLinecap="round" strokeLinejoin="round" />
      <path d={d} fill="none" stroke={look.color} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" />
      {look.striped && (
        <path d={d} fill="none" stroke="#ffffff" strokeWidth={4} strokeDasharray="7 7" strokeLinejoin="round" />
      )}
    </g>
  );
}

function WireColor({ look }: { look: Look }) {
  return (
    <span className="inline-flex items-center gap-2">
      <span
        aria-hidden
        className="inline-block w-5 h-2.5 rounded-sm ring-1 ring-black/20"
        style={{
          background: look.striped
            ? `repeating-linear-gradient(90deg, ${look.color} 0 5px, #fff 5px 10px)`
            : look.color,
        }}
      />
      {look.name}
    </span>
  );
}

/** A machine column. `wire` is its conductor in the current harness; the
 * breakout position, and so the cable conductor, is the same in both layouts. */
type Column = { slot: number; product?: Product; wire?: Wire; gpioPin?: number };

export function WiringSection() {
  const { machineId } = useMachine();
  const { data: status } = useQuery({
    queryKey: ["machineStatus", machineId],
    queryFn: () => getMachineStatus(machineId),
    enabled: !!machineId,
  });
  // Highlighted breakout position, or "common" for both commons.
  const [active, setActive] = useState<number | "common" | null>(null);
  const [layout, setLayout] = useState<Layout>("current");
  const wires = layout === "current" ? CURRENT : PROPOSED;

  const columns: Column[] = (status?.slots ?? [])
    .map((s) => ({
      slot: s.slot.position,
      product: s.product,
      gpioPin: s.slot.gpioPin,
      wire: CURRENT.find((w) => w.bcm != null && w.bcm === s.slot.gpioPin),
    }))
    .sort((a, b) => a.slot - b.slot);
  const columnOf = (w: Wire) => columns.find((c) => c.wire?.position === w.position);
  const unmapped = columns.filter((c) => !c.wire);

  const isDim = (w: Wire) =>
    active != null && !(active === "common" ? w.role === "common" : active === w.position);
  const hover = (key: number | "common" | null) => ({
    onMouseEnter: () => setActive(key),
    onMouseLeave: () => setActive(null),
  });

  const commons = wires.filter((w) => w.role === "common");
  const busTop = colY(0) - 30;
  const busBottom = colY(Math.max(columns.length, 1) - 1) + 30;

  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-3 mb-4">
        <div className="min-w-0 flex-1">
          <p className="text-xs font-medium uppercase tracking-wider text-text-tertiary">Wiring</p>
          <p className="mt-1 text-sm text-text-secondary">
            {layout === "current"
              ? "Pi GPIO header to the RJ45 breakout to each column's cam contact, as wired now."
              : "A planned rewire: every jumper on header pins 33–40, colored in order by breakout position. Not wired yet."}{" "}
            Hover a wire to trace it. Every contact closes its signal pin to ground; the Pi's internal pull-ups hold
            the pins high.
          </p>
        </div>
        <div
          className="inline-flex shrink-0 rounded-lg border border-border-primary p-0.5 text-xs"
          role="group"
          aria-label="Wiring layout"
        >
          {(["current", "proposed"] as const).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setLayout(v)}
              aria-pressed={layout === v}
              className={`px-3 py-2 sm:px-2.5 sm:py-1 rounded-md font-medium capitalize transition-colors duration-150 ${
                layout === v ? "bg-accent-soft text-accent" : "text-text-tertiary hover:text-text-primary"
              }`}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      <div className="overflow-x-auto">
        <svg
          viewBox={`0 0 ${W} ${H}`}
          className="w-full min-w-[720px] h-auto select-none"
          role="img"
          aria-label="Wiring diagram from the Pi's GPIO header through the RJ45 breakout to the soda machine's cam contacts; the table below lists every connection"
          fontFamily="inherit"
        >
          {/* Section titles */}
          <g fontSize={13} fontWeight={600} fill="#57534e">
            <text x={(ODD_X + EVEN_X) / 2} y={28} textAnchor="middle">Raspberry Pi header</text>
            <text x={BO_X + BO_W / 2} y={28} textAnchor="middle">RJ45 breakout</text>
            <text x={SW_X + 60} y={28} textAnchor="middle">Soda machine</text>
          </g>
          <text x={(ODD_X + EVEN_X) / 2} y={46} textAnchor="middle" fontSize={10} fill="#a8a29e">
            pin 1 at the end away from the USB ports
          </text>
          <text x={BO_X + BO_W / 2} y={46} textAnchor="middle" fontSize={10} fill="#a8a29e">
            cable to the machine plugs in here
          </text>
          <text x={SW_X + 60} y={46} textAnchor="middle" fontSize={10} fill="#a8a29e">
            one bare-wire contact per column
          </text>

          {/* Header body */}
          <rect
            x={ODD_X - PIN_R - 8}
            y={HEADER_TOP - PIN_R - 8}
            width={EVEN_X - ODD_X + 2 * (PIN_R + 8)}
            height={19 * ROW + 2 * (PIN_R + 8)}
            rx={6}
            fill="#3f3f46"
          />

          {/* Pi → breakout jumpers */}
          {wires.map((w) => {
            const exit = headerExit(w.physical);
            const label = w.bcm == null ? `GND · pin ${w.physical}` : `GPIO${w.bcm} · pin ${w.physical}`;
            return (
              <g key={`pi-${w.position}`} {...hover(w.role === "common" ? "common" : w.position)} className="cursor-pointer">
                <Conductor
                  d={`${exit.d} ${curveTo(EXIT_X, exit.y, LEAD_X, termY(w.position))} L ${BO_X + 14} ${termY(w.position)}`}
                  look={jumperLook(w)}
                  dim={isDim(w)}
                />
                <text
                  x={BO_X - 8}
                  y={termY(w.position) - 8}
                  textAnchor="end"
                  fontSize={10}
                  fontWeight={500}
                  fill="#44403c"
                  paintOrder="stroke"
                  stroke="#ffffff"
                  strokeWidth={3}
                  opacity={isDim(w) ? 0.25 : 1}
                >
                  {label}
                </text>
              </g>
            );
          })}

          {/* Header pins, drawn over the wires' ends */}
          {Array.from({ length: 40 }, (_, i) => {
            const n = i + 1;
            const w = wires.find((x) => x.physical === n);
            const cx = n % 2 ? ODD_X : EVEN_X;
            return (
              <g key={n} opacity={w && isDim(w) ? 0.35 : 1}>
                <circle
                  cx={cx}
                  cy={pinY(n)}
                  r={PIN_R}
                  fill={w ? jumperLook(w).color : "#e7e5e4"}
                  stroke={w ? (LIGHT.has(jumperLook(w).color) ? "#44403c" : "#ffffff") : "#a8a29e"}
                  strokeWidth={w ? 2 : 1}
                />
                <text
                  x={cx}
                  y={pinY(n) + 3.5}
                  textAnchor="middle"
                  fontSize={9}
                  fontWeight={w ? 700 : 400}
                  fill={w && !LIGHT.has(jumperLook(w).color) ? "#ffffff" : "#1c1917"}
                >
                  {n}
                </text>
              </g>
            );
          })}

          {/* Breakout board */}
          <rect x={BO_X} y={BO_TOP} width={BO_W} height={8 * BO_PITCH} rx={8} fill="#1e3a5f" />
          <rect x={BO_X + BO_W - 8} y={BO_TOP + 8} width={JACK_W} height={8 * BO_PITCH - 16} rx={4} fill="#d6d3d1" stroke="#a8a29e" />
          <text
            x={BO_X + BO_W + 7}
            y={BO_TOP + 4 * BO_PITCH}
            fontSize={10}
            fill="#57534e"
            textAnchor="middle"
            transform={`rotate(90 ${BO_X + BO_W + 7} ${BO_TOP + 4 * BO_PITCH})`}
          >
            RJ45 jack
          </text>
          {wires.map((w) => (
            <g key={`bo-${w.position}`} opacity={isDim(w) ? 0.35 : 1}>
              <circle cx={BO_X + 14} cy={termY(w.position)} r={7} fill="#fbbf24" stroke="#78350f" />
              <text x={BO_X + 30} y={termY(w.position) + 4} fontSize={12} fontWeight={700} fill="#ffffff">
                {w.position}
              </text>
              <text x={BO_X + 46} y={termY(w.position) + 4} fontSize={9} fill="#cbd5e1">
                {w.colorName}
              </text>
            </g>
          ))}

          {/* Breakout → machine: signals to each column's contact */}
          {columns.map((c, i) => {
            if (!c.wire) return null;
            const y = colY(i);
            const w = c.wire;
            return (
              <g key={`col-${c.slot}`} {...hover(w.position)} className="cursor-pointer">
                <Conductor d={curve(JACK_OUT, termY(w.position), SW_X, y)} look={cableLook(w)} dim={isDim(w)} />
              </g>
            );
          })}

          {/* Breakout → machine: the two commons to the shared ground bus */}
          {commons.map((w, i) => {
            const end = i === 0 ? busTop : busBottom;
            return (
              <g key={`common-${w.position}`} {...hover("common")} className="cursor-pointer">
                <Conductor d={curve(JACK_OUT, termY(w.position), BUS_X, end)} look={cableLook(w)} dim={isDim(w)} />
              </g>
            );
          })}
          <g opacity={active != null && active !== "common" ? 0.35 : 1} {...hover("common")}>
            <line x1={BUS_X} y1={busTop} x2={BUS_X} y2={busBottom} stroke="#44403c" strokeWidth={4} strokeLinecap="round" />
            <text x={BUS_X + 8} y={busBottom + 16} fontSize={10} fill="#57534e">
              common (ground) to every contact
            </text>
          </g>

          {/* Spare conductor, unused */}
          {wires.filter((w) => w.role === "spare").map((w) => {
            const y = termY(w.position);
            return (
              <g key="spare" {...hover(w.position)} className="cursor-pointer">
                <Conductor d={`M ${JACK_OUT} ${y} L ${JACK_OUT + 60} ${y}`} look={cableLook(w)} dim={isDim(w)} />
                <circle cx={JACK_OUT + 66} cy={y} r={5} fill="#ffffff" stroke="#57534e" strokeWidth={1.5} />
                <text x={JACK_OUT + 76} y={y + 4} fontSize={10} fill="#78716c">
                  spare
                </text>
              </g>
            );
          })}

          {/* Contacts */}
          {columns.map((c, i) => {
            const y = colY(i);
            const dim = active != null && !(c.wire && (active === c.wire.position || active === "common"));
            return (
              <g key={`sw-${c.slot}`} opacity={dim ? 0.35 : 1} {...(c.wire ? hover(c.wire.position) : {})}>
                {/* Open contact: two bare wires the cam arm presses together */}
                <circle cx={SW_X} cy={y} r={4} fill="#44403c" />
                <line x1={SW_X} y1={y} x2={SW_X + SW_GAP - 4} y2={y - 12} stroke="#44403c" strokeWidth={3} strokeLinecap="round" />
                <circle cx={SW_X + SW_GAP} cy={y} r={4} fill="#44403c" />
                <line x1={SW_X + SW_GAP} y1={y} x2={BUS_X} y2={y} stroke="#44403c" strokeWidth={3} />
                <circle cx={BUS_X} cy={y} r={4} fill="#44403c" />
                <text x={BUS_X + 14} y={y - 3} fontSize={13} fontWeight={600} fill="#1c1917">
                  Column {c.slot}
                  {UNCONFIRMED_SLOTS.has(c.slot) ? " *" : ""}
                </text>
                <text x={BUS_X + 14} y={y + 13} fontSize={11} fill="#57534e">
                  {c.product?.name ?? "No product"}
                </text>
                {c.product?.color && (
                  <circle cx={BUS_X + 14 + 4} cy={y + 26} r={4} fill={c.product.color} />
                )}
              </g>
            );
          })}
        </svg>
      </div>

      <div className="mt-4 overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-xs text-text-tertiary">
              <th className="py-2 pr-3 font-medium">Breakout</th>
              <th className="py-2 pr-3 font-medium">Jumper (Pi to breakout)</th>
              <th className="py-2 pr-3 font-medium">Cable wire</th>
              <th className="py-2 pr-3 font-medium">Pi pin</th>
              {layout === "proposed" && <th className="py-2 pr-3 font-medium">Pin now</th>}
              <th className="py-2 pr-3 font-medium">GPIO</th>
              <th className="py-2 font-medium">Goes to</th>
            </tr>
          </thead>
          <tbody>
            {wires.map((w) => {
              const col = columnOf(w);
              return (
                <tr
                  key={w.position}
                  className={`border-t border-border-primary transition-colors ${
                    active != null && !isDim(w) ? "bg-accent-soft" : ""
                  }`}
                  {...hover(w.role === "common" ? "common" : w.position)}
                >
                  <td className="py-2 pr-3 tabular-nums">{w.position}</td>
                  <td className="py-2 pr-3">
                    <WireColor look={jumperLook(w)} />
                  </td>
                  <td className="py-2 pr-3">
                    <WireColor look={cableLook(w)} />
                  </td>
                  <td className="py-2 pr-3 tabular-nums">{w.physical}</td>
                  {layout === "proposed" && (
                    <td className="py-2 pr-3 tabular-nums text-text-tertiary">
                      {CURRENT[w.position - 1].physical}
                      {CURRENT[w.position - 1].bcm != null ? ` (GPIO${CURRENT[w.position - 1].bcm})` : " (GND)"}
                    </td>
                  )}
                  <td className="py-2 pr-3 tabular-nums">{w.bcm == null ? "GND" : w.bcm}</td>
                  <td className="py-2">
                    {col ? (
                      <span className="inline-flex items-center gap-2">
                        <ProductDot color={col.product?.color} />
                        Column {col.slot} · {col.product?.name ?? "no product"}
                        {UNCONFIRMED_SLOTS.has(col.slot) ? " *" : ""}
                      </span>
                    ) : w.role === "common" ? (
                      "Common side of every contact"
                    ) : w.role === "spare" ? (
                      <span className="text-text-tertiary">Spare, not used</span>
                    ) : (
                      <span className="text-text-tertiary">No slot uses GPIO{w.bcm}</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <ul className="mt-4 space-y-1 text-xs text-text-tertiary">
        <li>* Columns 1 and 2 (both Diet Mountain Dew) may be the other way round; that hasn't been confirmed.</li>
        <li>
          The breakout and the cable's plug are both wired T568B (standard Ethernet type B). Cable colors are
          the Pi-side ones; the cable's colors don't match end to end, so don't go by color at the machine.
          {layout === "current" &&
            " Jumpers between the header and the breakout match the cable's colors except where the table says otherwise."}
        </li>
        <li>Nothing connects to the 3.3V pins (1, 17) or the 5V pins (2, 4).</li>
        {layout === "proposed" && (
          <li>
            To move to this layout: change each column's GPIO pin in Slot Configuration and in{" "}
            <code>piagent.yaml</code> on the Pi to the GPIO numbers above, move the jumpers, then restart the
            container. Counting stops while the jumpers are off, so do it outside lunch hours. Jumpers follow the
            resistor color code (brown, red, orange, yellow, green, blue, purple, grey), the order of a rainbow
            Dupont ribbon.
          </li>
        )}
        {unmapped.map((c) => (
          <li key={c.slot} className="text-status-low">
            Column {c.slot} is configured for {c.gpioPin != null ? `GPIO${c.gpioPin}` : "no GPIO pin"}, which isn't
            on this harness.
          </li>
        ))}
      </ul>
    </Card>
  );
}
