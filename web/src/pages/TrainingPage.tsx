import { useParams } from "react-router-dom";
import { TrainingProfileSection } from "../components/TrainingProfileSection";
import { PageSeo } from "../components/PageSeo";
import { PageStack } from "../components/ui/Stack";
export function TrainingPage({ owner = false }: { owner?: boolean }) {
  const { handle = "" } = useParams();
  return <PageStack><PageSeo title={`${owner ? "Your training" : `@${handle} · Training`} · AimMod`} description="osu! training history, skills and practice progress." noindex={owner} /><h1 className="text-2xl font-semibold">{owner ? "Your training" : `@${handle} · Training`}</h1><TrainingProfileSection handle={handle} owner={owner} /></PageStack>;
}
