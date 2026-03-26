import { useEffect, useState } from 'react'
import QRCodeLib from 'qrcode'

type QRCodeProps = {
  value: string
  size?: number
  className?: string
}

export function QRCode({ value, size = 192, className = '' }: QRCodeProps) {
  const [dataURL, setDataURL] = useState('')

  useEffect(() => {
    let active = true

    async function renderCode() {
      if (!value.trim()) {
        if (active) setDataURL('')
        return
      }
      try {
        const next = await QRCodeLib.toDataURL(value, {
          margin: 1,
          width: size,
          color: {
            dark: '#17212f',
            light: '#ffffff',
          },
        })
        if (active) setDataURL(next)
      } catch {
        if (active) setDataURL('')
      }
    }

    void renderCode()
    return () => {
      active = false
    }
  }, [size, value])

  if (!dataURL) return null

  return (
    <img
      src={dataURL}
      alt="Authenticator QR code"
      width={size}
      height={size}
      className={className}
    />
  )
}
