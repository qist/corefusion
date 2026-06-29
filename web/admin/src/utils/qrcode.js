import QRCode from 'qrcode'

export async function generateQrDataUrl(text, options = {}) {
  if (!text) return ''

  // Yield once so loading state can render before QR encoding starts.
  await new Promise(resolve => {
    if (typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(() => resolve())
      return
    }
    setTimeout(resolve, 0)
  })

  return QRCode.toDataURL(text, {
    errorCorrectionLevel: options.errorCorrectionLevel || 'L',
    margin: options.margin ?? 1,
    width: options.width || 240,
    type: 'image/png',
  })
}

export function isQrDataTooLargeError(error) {
  const message = error?.message || String(error || '')
  return /code length overflow|data too long|input too long|too big/i.test(message)
}
