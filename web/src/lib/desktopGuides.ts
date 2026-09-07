export type DesktopGuide = {
  slug: string;
  title: string;
  summary: string;
  location: string;
  sections: { title: string; paragraphs?: string[]; steps?: string[] }[];
};

export const desktopGuides: DesktopGuide[] = [
  {
    slug: "getting-started", title: "Set up AimMod", location: "Settings",
    summary: "Choose your osu! client, check your library, and take your first play into coaching.",
    sections: [
      { title: "Follow the setup tour", paragraphs: ["The first-run flow has four steps: Welcome, Your settings, App tour, and Ready. Your settings contains the actual client, account, sharing, automatic-practice, and startup preferences, so your choices carry through into the app.", "Use Finish setup when you are ready, or Skip setup to explore on your own. You can return at any time from Settings → Startup & introduction → Open setup & app tour. Opening a replay or an app link takes you to that task without interrupting it with the tour."] },
      { title: "Your first session", steps: ["Install AimMod and open it alongside your existing osu! installation.", "In Settings, choose your Open and install destination: Auto, osu!stable, or osu!lazer. This controls where maps and practice sets open.", "Open Beatmaps and confirm a difficulty you have installed appears. Then open Replays and choose a completed play with a saved replay.", "Open Coaching → Find a map, choose that play, and prepare a practice set. Play a section, then return to My coaching to review it."] },
      { title: "Startup sound", paragraphs: ["In Settings → Startup & introduction, use Startup sound to turn the AimMod theme on or off. Preview startup sound lets you hear it immediately. It follows the app's audio and mute settings, and does not play when AimMod launches directly into a replay or app link."] },
      { title: "Choose what happens automatically", paragraphs: ["Automatic practice and automatic sharing are separate settings. You can practise locally without automatically publishing your plays. Review both before turning either on.", "Linking an AimMod Hub account is used for sharing. Your osu! account and your Hub account have different roles: check which account is selected when looking at online scores or personal progress."] },
      { title: "A useful first goal", paragraphs: ["Pick a map you can finish but cannot play cleanly yet. Work on one short section at a comfortable speed, then replay the original difficulty with the same mods. That gives you a clearer comparison than switching maps, speed, and mods after every attempt."] },
    ],
  },
  {
    slug: "library-and-clients", title: "Beatmaps and osu! clients", location: "Beatmaps · Settings",
    summary: "Find the right difficulty and send maps to the client you actually play.",
    sections: [
      { title: "Browse by difficulty", paragraphs: ["Beatmaps includes installed and online maps. Select the exact difficulty before inspecting its skill demands or opening it in osu!. Two difficulties in the same mapset can need very different skills.", "An online score does not necessarily mean its map or replay is installed. Practice generation needs the original difficulty and its audio locally. Replay-specific coaching also needs replay evidence."] },
      { title: "Choose a destination", steps: ["Open Settings → Open and install destination.", "Choose osu!stable or osu!lazer if you want a specific client. Use Auto when you want AimMod to select the available destination.", "Open a map from AimMod and check that it arrives in the client you intended before importing a practice set."] },
      { title: "When a map cannot be prepared", paragraphs: ["Install the exact difficulty in osu!, including its audio, then use Retry loading this map in the practice screen. A different difficulty with the same song title is not a substitute.", "If it still fails, verify that the difficulty plays in osu! and that AimMod is reading the installation where you installed it."] },
    ],
  },
  {
    slug: "replays-and-statistics", title: "Replays and statistics", location: "Replays · Statistics",
    summary: "Connect what happened in a play with the patterns in your longer-term results.",
    sections: [
      { title: "Inspect a play", steps: ["In Replays, select the play you want to understand.", "Watch the difficult section and inspect its judgement and miss information. Check whether the cursor arrives late, tapping drifts, or the same pattern repeatedly breaks your combo.", "Take that play into Coaching to practise the section rather than retrying the entire map immediately."] },
      { title: "Compare like with like", paragraphs: ["In Statistics, narrow the history to the player and setup you want to compare. Keep the map difficulty, mods, speed, and scoring client consistent when judging progress.", "Accuracy is the proportion of available judgement value you earned. Misses count broken hits. A higher accuracy on one attempt can be encouraging, but several cleaner attempts are stronger evidence of repeatable improvement."] },
      { title: "A score is not always a replay", paragraphs: ["Online score details can be available without a replay file. A score summary can show the result, but it cannot show exactly where your cursor or tapping went wrong. Use a saved local replay for section-level evidence.", "Best-score lists are selected results, not every attempt you made. Do not read them as a pass rate or as proof that a map is easy for you."] },
    ],
  },
  {
    slug: "coaching", title: "Coach a beatmap", location: "Coaching → Find a map / My coaching",
    summary: "Move from a specific problem to practice, section progress, and an original-map retest.",
    sections: [
      { title: "Start with a play", steps: ["Open Coaching → Find a map and choose an eligible completed manual play.", "Review the suggested focus, then select Prepare practice set.", "Choose the whole set to practise several detected problem sections, or a single section for a narrower session. Set a comfortable speed and adjust repetitions and lead-in if needed.", "Create the set and use Open in osu! to import it. Its practice sections appear as separate difficulties.", "Play the sections, then return to Coaching → My coaching and open the original beatmap's coaching page."] },
      { title: "Find everything for one map", paragraphs: ["Overview gives you a quick view of attempts and section coverage. Sections shows accuracy, misses, consistency, trends, and expandable attempt details. Practice sets lists saved versions you can reopen. Original map compares later plays with your starting results.", "My coaching is the way back to maps you already worked on. You do not need to create a new plan to reopen a saved set or see its progress."] },
      { title: "Use the section statistics", paragraphs: ["Start with a section that has repeated misses or inconsistent accuracy. Practise it slowly enough to stay controlled. When several attempts are clean, raise the speed or return to the original map.", "Practice comparisons are grouped by setup. After six completed attempts, AimMod can compare your first three with your latest three. Accuracy spread describes how much your accuracy varies between attempts; a smaller spread means those results were steadier. It is not a millisecond tapping measurement.", "Sections suggested from map patterns without replay evidence are practice opportunities, not confirmed mistakes you made."] },
      { title: "Check whether it carries over", paragraphs: ["Replay the original difficulty with the same mods and scoring setup. The original-map comparison needs at least three completed baseline plays and three completed retests after practice. Until then, follow the prompt to collect more results.", "Look for both higher accuracy and fewer misses over several runs. A before-and-after improvement shows a change in results; it does not prove that one drill alone caused it."] },
    ],
  },
  {
    slug: "automatic-practice", title: "Automatic practice and cleanup", location: "Settings → Automatic practice",
    summary: "Keep a manageable practice collection that follows your recent plays.",
    sections: [
      { title: "Turn it on", steps: ["Open Settings → Automatic practice and choose On. It starts off.", "Keep AimMod open while you play so it can read new results and maintain the practice collection.", "Open Coaching → My coaching to review the sets, then open a set in osu! when you want to play it."] },
      { title: "What AimMod maintains", paragraphs: ["AimMod keeps up to five active automatic practice maps, using recent replay evidence to choose sections. As more original-map results arrive, it can revise the practice set rather than keeping the same exercises forever.", "The AimMod practice collection is managed inside AimMod. Importing a set into osu! is a separate action. AimMod does not sync an osu! collection or remove already imported copies from osu!."] },
      { title: "Cleanup choices", paragraphs: ["With Automatic cleanup, mastered sets and sets inactive for 30 days are archived. Old generated files can be removed after a further seven days. Progress history is retained.", "Favourites and manually created sets are protected from automatic cleanup. Choose Keep all sets if you want to keep the generated files. Removing a generated archive from AimMod does not delete the original beatmap."] },
      { title: "When no set appears", paragraphs: ["Check that automatic practice is on, that AimMod has remained open, and that recent manual plays have usable replay evidence and an installed source map. A new score alone may not provide enough information to choose meaningful sections."] },
    ],
  },
  {
    slug: "pp-targets", title: "Find a PP target", location: "PP Targets",
    summary: "Choose achievable targets from your skill fit and the accuracy you can sustain.",
    sections: [
      { title: "Choose a realistic target", steps: ["Open PP Targets and use the scoring setup you intend to play.", "Inspect the exact difficulty, its skill demands, and the suggested fit against your results.", "Compare PP at an accuracy you can repeat, rather than choosing only by the 100% value.", "Install or open the map, try it, and use Coaching if one section repeatedly stops you."] },
      { title: "Read estimates correctly", paragraphs: ["An FC estimate assumes a full combo at the stated accuracy and setup. Misses, mods, scoring differences, and the actual judgement distribution can change the final PP.", "Skill fit helps you choose what to try. It is not a promise of a full combo in a particular number of attempts. Account PP gain can also be smaller than the play's displayed PP because scores contribute with weighting."] },
    ],
  },
  {
    slug: "skins", title: "Skins and playback", location: "Skins · Replays",
    summary: "Choose a readable skin and check how it looks during actual play.",
    sections: [
      { title: "Choose a skin", paragraphs: ["Open Skins to browse installed osu!stable and osu!lazer skins. Preview the skin against gameplay so you can judge its cursor, hit objects, approach circles, and contrast together.", "For custom combinations, the website's Skin builder lets you choose components and export a skin. Import the exported skin into the osu! client you play."] },
      { title: "Keep practice readable", paragraphs: ["Use a cursor and hit sounds you can follow comfortably. When checking tapping, keep music and hit sounds audible enough to hear their relationship. Avoid changing your skin and several gameplay settings at once while comparing practice sessions."] },
    ],
  },
  {
    slug: "sharing-and-privacy", title: "Sharing and privacy", location: "Settings → account and sharing",
    summary: "Decide which plays leave your device and what a shared result includes.",
    sections: [
      { title: "Link your Hub account", steps: ["In Settings, choose Link account and follow the browser link and code prompt.", "Confirm the linked account shown in AimMod.", "Review visibility and the options Include replay file and Include judgement analysis before sharing."] },
      { title: "Automatic sharing is optional", paragraphs: ["Automatically share new qualifying plays starts off. When enabled, the minimum PP and accuracy settings determine which new plays qualify. The replay-file and judgement-analysis options control whether those extra details are included.", "The initial sharing visibility is Public. Choose your intended visibility before publishing a result. Local practice tracking and automatic practice do not require turning on automatic sharing."] },
      { title: "Check the queue", paragraphs: ["Use the sharing queue in Settings to check whether a share has completed or needs attention. If account linking fails, retry the link flow and confirm the intended Hub account in your browser.", "Turning off automatic sharing changes future sharing behaviour. It does not by itself remove results you already published. Review shared results on the Hub separately."] },
    ],
  },
  {
    slug: "troubleshooting", title: "Troubleshooting", location: "Start with the affected screen",
    summary: "Resolve missing maps, practice results, imports, and unexpected comparisons.",
    sections: [
      { title: "The source map cannot be loaded", paragraphs: ["Install the exact original difficulty and its audio in osu!. Check that it plays there, then return to the practice page and select Retry loading this map. An online score or another difficulty of the same song is not enough."] },
      { title: "A practice attempt is missing", paragraphs: ["Finish a play on the generated practice difficulty, keep AimMod open, and use Refresh results in My coaching. Automatic result checks happen roughly every 30 seconds.", "Check the player account and the exact practice-set version. An edited or replaced practice difficulty may no longer match the saved set. Older exports without tracking links can be reopened, but do not have linked section progress."] },
      { title: "The original-map comparison is empty", paragraphs: ["Use the original difficulty, the same mods and scoring client, and complete several plays. Practice-section results are recorded separately. AimMod needs enough completed original plays before and after practice to make the comparison."] },
      { title: "The wrong osu! client opens", paragraphs: ["Set Settings → Open and install destination explicitly to osu!stable or osu!lazer, then try opening the set again. If a file association in your operating system opens a different client, import the archive from the intended client."] },
      { title: "An archived set has no Open button", paragraphs: ["Its generated files were cleaned up while its progress history was retained. Create a new practice set from the original map. To retain future automatic sets, favourite them or select Keep all sets in cleanup settings."] },
      { title: "Ask for help", paragraphs: ["Describe what you selected, what you expected, and what happened. Include the AimMod version and osu! client. Share only the relevant error or a cropped screenshot; remove account details or private paths before posting."] },
    ],
  },
];

export function searchDesktopGuides(query: string) {
  const words = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
  return desktopGuides.filter(guide => {
    const text = [guide.title, guide.summary, guide.location, ...guide.sections.flatMap(section => [section.title, ...(section.paragraphs ?? []), ...(section.steps ?? [])])].join(" ").toLocaleLowerCase();
    return words.every(word => text.includes(word));
  });
}
