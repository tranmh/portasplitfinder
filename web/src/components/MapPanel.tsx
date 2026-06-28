"use client";

import dynamic from "next/dynamic";
import type { Center, StoreAvailability } from "@/lib/api-client";
import type { Messages } from "@/i18n";

// Leaflet needs `window`, so MapView is imported with ssr:false.
const MapView = dynamic(() => import("./MapView").then((m) => m.MapView), {
  ssr: false,
  loading: () => <MapLoading />,
});

function MapLoading() {
  return (
    <div className="flex h-full w-full items-center justify-center bg-slate-100 text-sm text-slate-500">
      …
    </div>
  );
}

export interface MapPanelProps {
  stores: StoreAvailability[];
  center: Center | null;
  radiusKm: number;
  messages: Messages;
}

export function MapPanel(props: MapPanelProps) {
  return (
    <div className="h-[60vh] w-full overflow-hidden rounded-lg border border-slate-200 shadow-sm lg:h-full">
      <MapView {...props} />
    </div>
  );
}

export default MapPanel;
