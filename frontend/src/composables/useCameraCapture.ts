import { onBeforeUnmount, ref } from 'vue'
import { MAX_CAPTURE_DIMENSION, scaleToFit } from '../utils/images'

type FacingMode = 'environment' | 'user'

/**
 * Shows the device camera (rear camera first) in a <video> element and takes still photos from it as
 * JPEG blobs. Stops the camera when the component unmounts.
 */
export function useCameraCapture() {
  const active = ref(false)
  const starting = ref(false)
  const error = ref<string | null>(null)
  const facingMode = ref<FacingMode>('environment')
  const supported = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia
  const secure = typeof window !== 'undefined' && window.isSecureContext

  let stream: MediaStream | null = null

  async function start(video: HTMLVideoElement) {
    if (starting.value) return
    if (!secure) {
      error.value = 'The camera needs a secure connection (HTTPS or localhost).'
      return
    }
    if (!supported) {
      error.value = 'This browser cannot use the camera.'
      return
    }
    stop()
    starting.value = true
    error.value = null
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: { ideal: facingMode.value }, width: { ideal: 1920 }, height: { ideal: 1080 } },
        audio: false,
      })
      video.srcObject = stream
      await video.play()
      active.value = true
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
      stop()
    } finally {
      starting.value = false
    }
  }

  /** Switches between the rear and front cameras. */
  async function flip(video: HTMLVideoElement) {
    facingMode.value = facingMode.value === 'environment' ? 'user' : 'environment'
    await start(video)
  }

  /** Takes a photo of the current frame, scaled so its longest side is at most MAX_CAPTURE_DIMENSION. */
  function capture(video: HTMLVideoElement, quality = 0.9): Promise<Blob> {
    const { width, height } = scaleToFit(video.videoWidth, video.videoHeight, MAX_CAPTURE_DIMENSION)
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d')
    if (!context || width === 0 || height === 0) {
      return Promise.reject(new Error('The camera is not ready yet.'))
    }
    context.drawImage(video, 0, 0, width, height)
    return new Promise((resolve, reject) => {
      canvas.toBlob(blob => (blob ? resolve(blob) : reject(new Error('Could not take a photo.'))), 'image/jpeg', quality)
    })
  }

  function stop() {
    active.value = false
    stream?.getTracks().forEach(track => track.stop())
    stream = null
  }

  onBeforeUnmount(stop)

  return { active, starting, error, facingMode, supported, secure, start, flip, capture, stop }
}
