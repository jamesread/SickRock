import { onBeforeUnmount, ref } from 'vue'

/** Formats asked of the detector: retail product codes plus common 2D and logistics codes. */
const BARCODE_FORMATS = ['ean_13', 'ean_8', 'upc_a', 'upc_e', 'code_128', 'code_39', 'itf', 'qr_code', 'data_matrix']

/** How often a camera frame is checked for a barcode. */
const SCAN_INTERVAL_MS = 150

type DetectedCode = { rawValue: string }
type Detector = { detect(source: HTMLVideoElement): Promise<DetectedCode[]> }
type DetectorConstructor = new (options?: { formats?: string[] }) => Detector

/**
 * Returns a function that reports whether a scanned code should count: the same code seen again within
 * windowMs (one item held in front of the camera) is ignored, and each sighting extends the window.
 */
export function createScanDebouncer(windowMs: number) {
  let lastCode = ''
  let lastSeen = Number.NEGATIVE_INFINITY
  return (code: string, now: number): boolean => {
    const repeat = code === lastCode && now - lastSeen < windowMs
    lastCode = code
    lastSeen = now
    return !repeat
  }
}

/**
 * The browser's BarcodeDetector when it supports any formats, otherwise the zxing-wasm ponyfill. The
 * ponyfill and its WebAssembly are loaded on first use and served by SickRock, not a CDN.
 */
async function loadDetector(): Promise<Detector> {
  const native = (globalThis as { BarcodeDetector?: DetectorConstructor & { getSupportedFormats?: () => Promise<string[]> } }).BarcodeDetector
  if (native?.getSupportedFormats) {
    const supported = await native.getSupportedFormats()
    const formats = BARCODE_FORMATS.filter(f => supported.includes(f))
    if (formats.length > 0) {
      return new native({ formats })
    }
  }
  const [{ BarcodeDetector, setZXingModuleOverrides }, { default: wasmUrl }] = await Promise.all([
    import('barcode-detector/ponyfill'),
    import('zxing-wasm/reader/zxing_reader.wasm?url'),
  ])
  setZXingModuleOverrides({
    locateFile: (path: string, prefix: string) => (path.endsWith('.wasm') ? wasmUrl : prefix + path),
  })
  return new BarcodeDetector({ formats: BARCODE_FORMATS as never })
}

/**
 * Scans barcodes from the device camera (rear camera preferred) into a <video> element, calling onCode
 * for each new code. Stops the camera when the component unmounts.
 */
export function useBarcodeCamera(onCode: (code: string) => void, debounceMs = 1500) {
  const active = ref(false)
  const starting = ref(false)
  const error = ref<string | null>(null)
  const supported = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia
  const secure = typeof window !== 'undefined' && window.isSecureContext

  let stream: MediaStream | null = null
  let timer: ReturnType<typeof setTimeout> | null = null
  const shouldCount = createScanDebouncer(debounceMs)

  async function start(video: HTMLVideoElement) {
    if (active.value || starting.value) return
    if (!secure) {
      error.value = 'The camera needs a secure connection (HTTPS or localhost).'
      return
    }
    if (!supported) {
      error.value = 'This browser cannot use the camera.'
      return
    }
    starting.value = true
    error.value = null
    try {
      const detector = await loadDetector()
      stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: { ideal: 'environment' } }, audio: false })
      video.srcObject = stream
      await video.play()
      active.value = true
      scanLoop(detector, video)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
      stop()
    } finally {
      starting.value = false
    }
  }

  function scanLoop(detector: Detector, video: HTMLVideoElement) {
    timer = setTimeout(async () => {
      if (!active.value) return
      try {
        if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) {
          const [first] = await detector.detect(video)
          const code = first?.rawValue?.trim()
          if (code && shouldCount(code, Date.now())) {
            onCode(code)
          }
        }
      } catch {
        // A frame that fails to decode is skipped; the next one is tried.
      }
      scanLoop(detector, video)
    }, SCAN_INTERVAL_MS)
  }

  function stop() {
    active.value = false
    if (timer) clearTimeout(timer)
    timer = null
    stream?.getTracks().forEach(track => track.stop())
    stream = null
  }

  onBeforeUnmount(stop)

  return { active, starting, error, supported, secure, start, stop }
}
