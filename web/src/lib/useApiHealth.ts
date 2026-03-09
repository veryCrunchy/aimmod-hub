import { useEffect, useState } from "react";
import { HealthRequest } from "../gen/aimmod/hub/v1/hub_pb";
import { hubClient } from "./api";

export function useApiHealth() {
  const [healthLabel, setHealthLabel] = useState("checking");
  const [statusLabel, setStatusLabel] = useState("API status");
  const [online, setOnline] = useState(false);

  useEffect(() => {
    let cancelled = false;

    hubClient
      .getHealth(new HealthRequest())
      .then((response) => {
        if (cancelled) return;
        setOnline(true);
        setStatusLabel("API online");
        setHealthLabel(`${response.service} · ${response.version}`);
      })
      .catch(() => {
        if (cancelled) return;
        setOnline(false);
        setStatusLabel("API offline");
        setHealthLabel("local dev not ready");
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return { healthLabel, statusLabel, online };
}
