import { useEffect, useRef } from 'react'
import * as maplibregl from 'maplibre-gl'
import { IconLayer } from 'deck.gl'
import { MapboxOverlay } from '@deck.gl/mapbox'
import type { Feature, Point } from 'geojson'
import 'maplibre-gl/dist/maplibre-gl.css'

type Ping = {
  Latitude: number
  Longitude: number
  Course: number
  Speed: number
  Heading: number | null
  Timestamp: string
}

type Update = {
  MMSI: number
  Name: string
  Ping: Ping
}

type VesselProps = {
  mmsi: number
  name: string
  speed: number
  course: number
  heading: number | null
  timestamp: string
  rotation: number
}

type VesselFeature = Feature<Point>

const ICON_SIZE = 64
const ICON_NAME = 'boat'

// Boat hull seen from above, drawn pointing north (up) so a rotation of 0
// means "heading due north" — matching how AIS expresses course/heading.
// Generated at runtime rather than shipped as an asset.
function makeBoatAtlas(size = ICON_SIZE): string {
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')!

  ctx.beginPath()
  ctx.moveTo(size * 0.5, size * 0.04) // bow
  ctx.lineTo(size * 0.74, size * 0.34) // starboard shoulder
  ctx.lineTo(size * 0.7, size * 0.94) // starboard quarter
  ctx.lineTo(size * 0.3, size * 0.94) // port quarter
  ctx.lineTo(size * 0.26, size * 0.34) // port shoulder
  ctx.closePath()

  ctx.fillStyle = '#1e88e5'
  ctx.fill()
  ctx.strokeStyle = '#0b3d66'
  ctx.lineWidth = size * 0.05
  ctx.stroke()

  return canvas.toDataURL()
}

/*
 * Triangle rendering, kept for the future style switch. When that becomes
 * stateful, this comes back as the alternative to the deck.gl boat layer
 * above — it renders through MapLibre's own symbol layer rather than deck.
 *
 * const TRIANGLE_ICON_ID = 'vessel-triangle'
 *
 * function makeTriangleImage(size = 48): ImageData {
 *   const canvas = document.createElement('canvas')
 *   canvas.width = size
 *   canvas.height = size
 *   const ctx = canvas.getContext('2d')!
 *
 *   ctx.beginPath()
 *   ctx.moveTo(size / 2, 4)
 *   ctx.lineTo(size - 8, size - 6)
 *   ctx.lineTo(size / 2, size - 14)
 *   ctx.lineTo(8, size - 6)
 *   ctx.closePath()
 *
 *   ctx.fillStyle = '#1e88e5'
 *   ctx.fill()
 *   ctx.strokeStyle = '#0b3d66'
 *   ctx.lineWidth = 3
 *   ctx.stroke()
 *
 *   return ctx.getImageData(0, 0, size, size)
 * }
 *
 * // In the map 'load' handler:
 * m.addImage(TRIANGLE_ICON_ID, makeTriangleImage(), { pixelRatio: 2 })
 * m.addSource('vessels', {
 *   type: 'geojson',
 *   data: { type: 'FeatureCollection', features: [] },
 * })
 * m.addLayer({
 *   id: 'vessels',
 *   type: 'symbol',
 *   source: 'vessels',
 *   layout: {
 *     'icon-image': TRIANGLE_ICON_ID,
 *     'icon-rotate': ['get', 'rotation'],
 *     'icon-rotation-alignment': 'map',
 *     'icon-allow-overlap': true,
 *     'icon-size': 0.5,
 *   },
 * })
 *
 * // And flush() pushed to the source instead of the deck overlay:
 * const source = map.current?.getSource('vessels') as maplibregl.GeoJSONSource
 * source?.setData({ type: 'FeatureCollection', features: [...] })
 */

function toFeature(update: Update): VesselFeature {
  const { MMSI, Name, Ping } = update
  const props: VesselProps = {
    mmsi: MMSI,
    name: Name,
    speed: Ping.Speed,
    course: Ping.Course,
    heading: Ping.Heading,
    timestamp: Ping.Timestamp,
    // Heading is the bow direction and is often null (the AIS 511
    // "not available" sentinel, converted to nil server-side). Course
    // is direction of travel over ground and is always present.
    rotation: Ping.Heading ?? Ping.Course,
  }

  return {
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [Ping.Longitude, Ping.Latitude] },
    properties: props,
  }
}

// Vessel names come from an untrusted external feed — a vessel can
// broadcast whatever it likes — so they can't go into popup HTML raw.
function escapeHtml(value: string): string {
  return value.replace(
    /[&<>"']/g,
    (c) =>
      ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;',
      })[c]!,
  )
}

function popupHtml(p: VesselProps, lon: number, lat: number): string {
  const name = p.name ? escapeHtml(p.name) : '(no name reported)'
  const heading = p.heading == null ? 'not reported' : `${p.heading}°`
  const seen = new Date(p.timestamp).toLocaleTimeString()

  return `
    <div style="font: 13px/1.5 system-ui, sans-serif; min-width: 180px">
      <div style="font-weight: 600; margin-bottom: 4px">${name}</div>
      <div style="color: #666">MMSI ${p.mmsi}</div>
      <hr style="border: none; border-top: 1px solid #eee; margin: 6px 0" />
      <div>Speed: ${p.speed} kn</div>
      <div>Course: ${p.course}°</div>
      <div>Heading: ${heading}</div>
      <div>Position: ${lat.toFixed(4)}, ${lon.toFixed(4)}</div>
      <div style="color: #666; margin-top: 4px">Last seen ${seen}</div>
    </div>
  `
}

function App() {
  const mapContainer = useRef<HTMLDivElement | null>(null)
  const map = useRef<maplibregl.Map | null>(null)
  const overlay = useRef<MapboxOverlay | null>(null)
  const vessels = useRef<Map<number, VesselFeature>>(new Map())
  const pendingFrame = useRef<number | null>(null)
  const popup = useRef<maplibregl.Popup | null>(null)
  const socket = useRef<WebSocket | null>(null)
  const boatAtlas = useRef<string>(makeBoatAtlas())

  useEffect(() => {
    if (!mapContainer.current || map.current) return

    const m = new maplibregl.Map({
      container: mapContainer.current,
      style: 'https://tiles.openfreemap.org/styles/liberty',
      center: [-80.14, 25.77], // Miami/Bahamas box, same as the ingest bounding box
      zoom: 9,
    })
    map.current = m

    m.on('load', () => {
      const deck = new MapboxOverlay({ layers: [] })
      overlay.current = deck
      m.addControl(deck)

      // Anything that arrived over the socket before the map was ready
      // is already sitting in the ref — push it now.
      flush()
      sendViewport()
    })

    // moveend fires once per gesture, not per frame. The server throttles
    // resubscribes anyway (aisstream drops the connection above 1/sec),
    // so no debounce needed here.
    m.on('moveend', sendViewport)

    return () => {
      m.remove()
      map.current = null
      overlay.current = null
    }
  }, [])

  function showPopup(props: VesselProps, lon: number, lat: number) {
    const m = map.current
    if (!m) return

    // Replace rather than stack — clicking vessel B while A's popup is
    // open should leave exactly one popup open.
    popup.current?.remove()
    popup.current = new maplibregl.Popup({ offset: 14 })
      .setLngLat([lon, lat])
      .setHTML(popupHtml(props, lon, lat))
      .addTo(m)
  }

  function buildLayer() {
    return new IconLayer<VesselFeature>({
      id: 'vessels',
      data: [...vessels.current.values()],
      getPosition: (f) => f.geometry.coordinates as [number, number],
      // deck.gl measures angle counter-clockwise; AIS course/heading is a
      // compass bearing (clockwise from north), hence the negation.
      getAngle: (f) => -(f.properties as VesselProps).rotation,
      getSize: 26,
      sizeUnits: 'pixels',
      iconAtlas: boatAtlas.current,
      iconMapping: {
        [ICON_NAME]: {
          x: 0,
          y: 0,
          width: ICON_SIZE,
          height: ICON_SIZE,
          mask: false,
        },
      },
      getIcon: () => ICON_NAME,
      pickable: true,
      onClick: (info) => {
        const f = info.object as VesselFeature | undefined
        if (!f) return
        const [lon, lat] = f.geometry.coordinates
        showPopup(f.properties as VesselProps, lon, lat)
      },
      onHover: (info) => {
        const canvas = map.current?.getCanvas()
        if (canvas) canvas.style.cursor = info.object ? 'pointer' : ''
      },
    })
  }

  // Coalesce updates to one layer rebuild per animation frame rather than
  // one per incoming ping.
  function flush() {
    overlay.current?.setProps({ layers: [buildLayer()] })
  }

  function scheduleFlush() {
    if (pendingFrame.current !== null) return
    pendingFrame.current = requestAnimationFrame(() => {
      pendingFrame.current = null
      flush()
    })
  }

  // Tell the backend what we're looking at so it can re-point its upstream
  // aisstream subscription at this area.
  function sendViewport() {
    const m = map.current
    const ws = socket.current
    if (!m || ws?.readyState !== WebSocket.OPEN) return

    const b = m.getBounds()
    ws.send(
      JSON.stringify({
        type: 'viewport',
        viewport: {
          west: b.getWest(),
          south: b.getSouth(),
          east: b.getEast(),
          north: b.getNorth(),
        },
      }),
    )
  }

  useEffect(() => {
    // Bare-minimum connection for now — no reconnect/backoff/status yet,
    // that's a deliberate later step, not an oversight.
    const ws = new WebSocket('ws://localhost:8080/ws')
    socket.current = ws

    // The map may already be loaded by the time the socket opens (or vice
    // versa) — whichever happens second triggers the first viewport send.
    ws.onopen = () => sendViewport()

    ws.onmessage = (event) => {
      const update: Update = JSON.parse(event.data)
      vessels.current.set(update.MMSI, toFeature(update))
      scheduleFlush()
    }

    return () => {
      socket.current = null
      ws.close()
      if (pendingFrame.current !== null) {
        cancelAnimationFrame(pendingFrame.current)
        pendingFrame.current = null
      }
    }
  }, [])

  return <div ref={mapContainer} style={{ width: '100vw', height: '100vh' }} />
}

export default App
