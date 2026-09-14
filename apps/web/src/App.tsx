import { useEffect, useRef } from 'react'
import * as maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'

function App() {
  const mapContainer = useRef<HTMLDivElement | null>(null)
  const map = useRef<maplibregl.Map | null>(null)

  useEffect(() => {
    if (!mapContainer.current || map.current) return

    map.current = new maplibregl.Map({
      container: mapContainer.current,
      style: 'https://tiles.openfreemap.org/styles/liberty',
      center: [-80.14, 25.77], // Miami/Bahamas box, same as the ingest bounding box
      zoom: 9,
    })

    return () => {
      map.current?.remove()
      map.current = null
    }
  }, [])

  useEffect(() => {
    // Bare-minimum connection for now — no reconnect/backoff/status yet,
    // that's a deliberate later step, not an oversight.
    const ws = new WebSocket('ws://localhost:8080/ws')

    ws.onmessage = (event) => {
      console.log(JSON.parse(event.data))
    }

    return () => {
      ws.close()
    }
  }, [])

  return <div ref={mapContainer} style={{ width: '100vw', height: '100vh' }} />
}

export default App
