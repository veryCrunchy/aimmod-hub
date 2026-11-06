import { PageStack } from "../components/ui/Stack";
import { PageHeader } from "../components/ui/PageHeader";
export function FairPlayPage() {
 return <PageStack><PageHeader title="Fair play" /><p>AimMod uses host-validated hits and replay checks to help assess runs. These checks do not guarantee that every form of cheating will be detected.</p><p>AimMod never changes KovaaK's ranked leaderboards. Hub uploads and public KovaaK's results remain separate.</p><p>Account verification proves account ownership; it is not a guarantee that every run is legitimate.</p><p>To report a suspicious run, <a href="/join" className="text-cyan hover:underline">contact the moderators on Discord</a> with its public run link and a short explanation. Do not post private account information.</p></PageStack>;
}
