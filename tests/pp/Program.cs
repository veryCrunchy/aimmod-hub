using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

static void Check(bool condition, string message) { if (!condition) throw new Exception(message); }
static void Reject(Action action) { try { action(); } catch (ArgumentException) { return; } throw new Exception("Invalid input was accepted"); }
var text = "osu file format v14\n\n[General]\nMode:0\n\n[Metadata]\nTitle:Example\nArtist:Example\nCreator:Example\nVersion:Synthetic\n\n[Difficulty]\nHPDrainRate:5\nCircleSize:4\nOverallDifficulty:8\nApproachRate:9\nSliderMultiplier:1.4\nSliderTickRate:1\n\n[TimingPoints]\n0,500,4,2,1,50,1,0\n\n[HitObjects]\n";
text += string.Join("\n", Enumerable.Range(0, 100).Select(i => i % 3 == 0
    ? $"64,192,{1000+i*500},2,0,L|384:192,1,280"
    : $"{64+i%2*320},192,{1000+i*500},1,0,0:0:0:0:"));
var bytes = Encoding.UTF8.GetBytes(text);
var request = new CalculationRequest(Convert.ToBase64String(bytes), Convert.ToHexString(MD5.HashData(bytes)), true, 98, JsonSerializer.SerializeToElement(Array.Empty<object>()), null);
CalculationResult Run(CalculationRequest r) => Calculator.Calculate(r, CancellationToken.None);
var lazer = Run(request);
var stable = Run(request with { Lazer = false });
Check(lazer.RulesetVersion >= 20260706, "Pre-rework ruleset is in use");
Check(lazer.Pp > 0 && lazer.MaxPp >= lazer.Pp, "Invalid FC estimate");
Check(Math.Abs(lazer.Pp - stable.Pp) > 0.01, "Stable and lazer scoring were conflated");
Check(Math.Abs(lazer.Pp - 43.74851421) < 0.000001 && Math.Abs(stable.Pp - 35.33898313) < 0.000001, "Pinned official ruleset regression values changed");
Check(Run(request with { Lazer = null }).Pp == lazer.Pp, "Lazer is not the default");
var dtMods = JsonSerializer.SerializeToElement(new[] { new { acronym = "DT", settings = new { speed_change = 1.2 } } });
var dt = Run(request with { Mods = dtMods });
Check(dt.Stars > lazer.Stars, "Custom-rate DT was ignored");
var actual = new ScoreInput(1, 0, true, request.Mods, new() { ["great"] = 90, ["ok"] = 5, ["meh"] = 2, ["miss"] = 3, ["slider_tail_hit"] = 30, ["large_tick_miss"] = 2 }, 42, 0.92, true, 123456, null);
var play = Run(request with { Input = actual });
Check(play.Pp < lazer.Pp, "Actual misses and combo were replaced with FC");
var early = Run(request with { Input = actual with { Passed = false, Statistics = new() { ["great"] = 1, ["slider_tail_hit"] = 1 }, MaxCombo = 1 } });
var later = Run(request with { Input = actual with { Passed = false, Statistics = new() { ["great"] = 90, ["slider_tail_hit"] = 30 }, MaxCombo = 90 } });
Check(early.Pp < later.Pp && early.Stars == later.Stars, "Partial PP or full-map difficulty is incorrect");
Check(Run(request with { Input = actual with { Lazer = false } }).Lazer == false, "Actual score mode was overridden");
Reject(() => Run(request with { Checksum = new string('0',32) }));
Reject(() => Run(request with { Input = actual with { Statistics = new() { ["great"] = 101 } } }));
Reject(() => Run(request with { Input = actual with { Lazer = null } }));
Reject(() => Run(request with { Accuracy = double.NaN }));
Reject(() => Run(request with { Mods = JsonSerializer.SerializeToElement(new[] { new { acronym = "DT" }, new { acronym = "HT" } }) }));
Console.WriteLine($"Passed official PP checks. Ruleset {lazer.RulesetVersion}; synthetic lazer {lazer.Pp:F8}, stable {stable.Pp:F8}, DT {dt.Pp:F8}.");

foreach (int mode in new[] {1, 2, 3}) {
    var modeBytes = Encoding.UTF8.GetBytes(text.Replace("Mode:0", $"Mode:{mode}"));
    var path = Path.Combine(Path.GetTempPath(), $"aimmod-mode-test-{Guid.NewGuid():N}.osu");
    try {
        File.WriteAllBytes(path, modeBytes);
        var working = new osu.Game.Beatmaps.FlatWorkingBeatmap(path);
        var ruleset = ModePerformance.CreateRuleset(mode);
        foreach (bool isLazer in new[] { true, false }) {
            osu.Game.Rulesets.Mods.Mod[] mods = [];
            if (!isLazer && ruleset.CreateMod<osu.Game.Rulesets.Mods.ModClassic>() is {} classic) mods = [classic];
            using var processor = ruleset.CreateScoreProcessor();
            processor.Mods.Value = mods;
            processor.ApplyBeatmap(working.GetPlayableBeatmap(ruleset.RulesetInfo, mods));
            var maxStatistics = processor.MaximumStatistics;
            if (mode == 3 && !isLazer) maxStatistics[osu.Game.Rulesets.Scoring.HitResult.Perfect] = working.GetPlayableBeatmap(ruleset.RulesetInfo, mods).HitObjects.Count;
            var perfect = maxStatistics.ToDictionary(pair => pair.Key.ToString(), pair => pair.Value);
            Console.WriteLine($"Mode {mode}, lazer {isLazer}: {JsonSerializer.Serialize(perfect)}");
            var modeScore = actual with { RulesetId = mode, Lazer = isLazer, Mods = request.Mods, Statistics = perfect, Accuracy = 1, MaxCombo = processor.MaximumCombo };
            var modeRequest = request with { Map = Convert.ToBase64String(modeBytes), Checksum = Convert.ToHexString(MD5.HashData(modeBytes)), Input = modeScore };
            var result = Run(modeRequest);
            Check(result.Pp > 0 && Math.Abs(result.Pp-result.MaxPp)<1e-8, "Perfect mode PP mismatch");
            var missStats = new Dictionary<string,int>(perfect);
            var hit = mode == 3 ? "Perfect" : "Great";
            missStats[hit] -= 1; missStats["Miss"] = 1;
            var missed = Run(modeRequest with { Input = modeScore with { Statistics = missStats, MaxCombo = 1, Accuracy = .98 } });
            Check(missed.Pp < result.Pp, "Mode misses ignored");
            Check(Run(modeRequest with { Input = modeScore with { Passed = false, Statistics = new() { [hit] = 1 }, MaxCombo = 1 } }).Pp < result.Pp, "Partial mode score inflated");
            Reject(() => Run(modeRequest with { Input = modeScore with { Statistics = new() { [hit] = 100001 } } }));
        }
    } finally { File.Delete(path); }
}
