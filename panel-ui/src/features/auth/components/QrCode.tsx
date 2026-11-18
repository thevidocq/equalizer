import { useEffect, useState } from "react";
import QRCode from "qrcode";

/** The otpauth link as a QR code, drawn in the browser (nothing leaves it). */
export function QrCode({ value, label }: { value: string; label: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    QRCode.toDataURL(value, { margin: 1, width: 192, color: { dark: "#1C2633", light: "#FFFFFF" } }).then((u) => live && setSrc(u));
    return () => { live = false; };
  }, [value]);
  return src ? <img src={src} alt={label} width={192} height={192} className="rounded-[var(--radius-control)] border border-rule" /> : null;
}
