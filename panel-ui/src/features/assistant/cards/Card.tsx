import type { Schemas } from "@/shared/api/client";
import { EventCard } from "./EventCard";
import { RefusalsCard } from "./RefusalsCard";
import { RouteCard } from "./RouteCard";

/** A card of the answer, built by the panel from real data. */
export function Card({ tenant, app, card }: { tenant: string; app: string; card: Schemas["AssistantCard"] }) {
  switch (card.kind) {
    case "event": return <EventCard tenant={tenant} app={app} card={card} />;
    case "refusals": return <RefusalsCard card={card} />;
    case "route": return <RouteCard tenant={tenant} app={app} card={card} />;
    default: return null;
  }
}
