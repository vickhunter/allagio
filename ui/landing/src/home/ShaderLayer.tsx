import { Shader, MeshGradient } from 'shaders/react'
// Free MeshGradient from the MIT 'shaders' package. Telemetry off: the page makes no network calls.
const STOPS = ['#f7f7ef', '#f8eae4', '#ecefe2', '#e8edfa', '#fdfdf7']
export default function ShaderLayer({ onReady, onUnavailable }: { onReady: () => void; onUnavailable: () => void }) {
  return (
    <Shader className="size-full" disableTelemetry onReady={onReady} onUnavailable={onUnavailable}>
      <MeshGradient colorA={STOPS[0]} colorB={STOPS[1]} stops={STOPS.map((color, i) => ({ color, position: i / (STOPS.length - 1) }))} count={5} smoothness={2.4} swirl={0.25} drift={0.3} speed={0.35} seed={16} />
    </Shader>
  )
}
