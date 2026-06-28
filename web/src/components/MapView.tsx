"use client";

import { useEffect, useMemo } from "react";
import {
  MapContainer,
  TileLayer,
  Marker,
  Popup,
  Circle,
  useMap,
} from "react-leaflet";
import L from "leaflet";
import "leaflet/dist/leaflet.css";

// Fix Leaflet's default marker icon paths (the classic Leaflet + webpack/Next gotcha:
// the bundler rewrites the asset URLs and Leaflet ends up requesting broken paths).
// We point Leaflet at the CDN-free copies that ship inside the leaflet package.
import iconRetinaUrl from "leaflet/dist/images/marker-icon-2x.png";
import iconUrl from "leaflet/dist/images/marker-icon.png";
import shadowUrl from "leaflet/dist/images/marker-shadow.png";

import type { Center, StoreAvailability } from "@/lib/api-client";
import type { Messages } from "@/i18n";
import { formatPrice } from "@/lib/format";

// Build a default icon once, using the resolved asset URLs.
const defaultIcon = L.icon({
  iconRetinaUrl: assetSrc(iconRetinaUrl),
  iconUrl: assetSrc(iconUrl),
  shadowUrl: assetSrc(shadowUrl),
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

// Distinct red marker for the user's PLZ center.
const centerIcon = L.icon({
  iconRetinaUrl: assetSrc(iconRetinaUrl),
  iconUrl: assetSrc(iconUrl),
  shadowUrl: assetSrc(shadowUrl),
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
  className: "psf-center-marker",
});

// Next's static-image imports can resolve to either a string or a
// StaticImageData object depending on config; normalise to a string.
function assetSrc(asset: unknown): string {
  if (typeof asset === "string") return asset;
  if (asset && typeof asset === "object" && "src" in asset) {
    return String((asset as { src: string }).src);
  }
  return "";
}

// Geographic center of Germany, used as the default view.
const GERMANY_CENTER: [number, number] = [51.1657, 10.4515];
const GERMANY_ZOOM = 6;

function RecenterOnChange({ center, radiusKm }: { center: Center | null; radiusKm: number }) {
  const map = useMap();
  useEffect(() => {
    if (!center) {
      map.setView(GERMANY_CENTER, GERMANY_ZOOM);
      return;
    }
    // Fit the view roughly to the requested radius.
    const latLng = L.latLng(center.lat, center.lon);
    const bounds = latLng.toBounds(radiusKm * 2 * 1000);
    map.fitBounds(bounds, { padding: [24, 24] });
  }, [center, radiusKm, map]);
  return null;
}

export interface MapViewProps {
  stores: StoreAvailability[];
  center: Center | null;
  radiusKm: number;
  messages: Messages;
}

export function MapView({ stores, center, radiusKm, messages }: MapViewProps) {
  const t = messages.map;

  const markers = useMemo(
    () =>
      stores.filter(
        (s) => Number.isFinite(s.store.lat) && Number.isFinite(s.store.lon),
      ),
    [stores],
  );

  return (
    <MapContainer
      center={GERMANY_CENTER}
      zoom={GERMANY_ZOOM}
      scrollWheelZoom
      className="h-full w-full"
      style={{ minHeight: "100%", minWidth: "100%" }}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />

      <RecenterOnChange center={center} radiusKm={radiusKm} />

      {center && (
        <>
          <Circle
            center={[center.lat, center.lon]}
            radius={radiusKm * 1000}
            pathOptions={{ color: "#0284c7", fillColor: "#0284c7", fillOpacity: 0.08 }}
          />
          <Marker position={[center.lat, center.lon]} icon={centerIcon}>
            <Popup>{t.yourLocation}</Popup>
          </Marker>
        </>
      )}

      {markers.map((entry) => {
        const anyInStock = entry.listings.some((l) => l.inStock);
        const cheapest = entry.listings
          .map((l) => l.priceCents)
          .filter((p): p is number => p !== null)
          .sort((a, b) => a - b)[0];
        const price = formatPrice(cheapest ?? null);
        return (
          <Marker
            key={entry.store.id}
            position={[entry.store.lat, entry.store.lon]}
            icon={defaultIcon}
          >
            <Popup>
              <div className="space-y-1">
                <p className="font-semibold">{entry.store.name}</p>
                <p className="text-xs text-slate-600">
                  {entry.store.plz} {entry.store.city}
                </p>
                <p>
                  <span
                    className={
                      anyInStock
                        ? "rounded bg-green-100 px-1.5 py-0.5 text-xs font-medium text-green-800"
                        : "rounded bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-600"
                    }
                  >
                    {anyInStock ? t.inStock : t.outOfStock}
                  </span>
                </p>
                {price && <p className="text-sm font-medium">{price}</p>}
              </div>
            </Popup>
          </Marker>
        );
      })}
    </MapContainer>
  );
}

export default MapView;
