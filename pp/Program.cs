using System.Security.Cryptography;
using System.Text.Json;
using Newtonsoft.Json;
using osu.Game.Beatmaps;
using osu.Game.Online.API;
using osu.Game.Rulesets.Mods;
using osu.Game.Rulesets.Osu;
using osu.Game.Rulesets.Osu.Difficulty;
using osu.Game.Rulesets.Scoring;
using osu.Game.Scoring;
using osu.Game.Tests.Beatmaps;

var builder = WebApplication.CreateBuilder(args);
builder.WebHost.UseUrls("http://127.0.0.1:5192");
builder.WebHost.ConfigureKestrel(options => options.Limits.MaxRequestBodySize = 6 * 1024 * 1024);
builder.Logging.ClearProviders();
var app = builder.Build();
var capacity = new SemaphoreSlim(2);
app.MapGet("/health", () => Results.Ok(new { engine = Calculator.Engine }));
app.MapPost("/calculate", async (CalculationRequest request, HttpContext context) => {
    if (!await capacity.WaitAsync(TimeSpan.FromSeconds(2), context.RequestAborted)) return Results.Json(new { error = "PP calculations are busy. Please retry." }, statusCode: 503);
    try {
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(context.RequestAborted);
        timeout.CancelAfter(TimeSpan.FromSeconds(20));
        return Results.Ok(await Task.Run(() => Calculator.Calculate(request, timeout.Token), timeout.Token));
    } catch (ArgumentException error) {
        return Results.BadRequest(new { error = error.Message });
    } catch (OperationCanceledException) {
        return Results.Json(new { error = "PP calculation timed out. Please retry." }, statusCode: 504);
    } catch {
        return Results.BadRequest(new { error = "PP could not be calculated for this beatmap and score." });
    } finally { capacity.Release(); }
});
app.Run();

public sealed record CalculationRequest(string Map, string Checksum, bool? Lazer, double Accuracy, JsonElement Mods, ScoreInput? Input);
public sealed record ScoreInput(int Version, int RulesetId, bool? Lazer, JsonElement Mods, Dictionary<string, int>? Statistics, int MaxCombo, double Accuracy, bool Passed, long TotalScore, long? LegacyTotalScore);
public sealed record CalculationResult(double Pp, double MaxPp, double Stars, int ObjectCount, double Accuracy, bool Lazer, string Engine, int RulesetVersion);

public static class Calculator {
    public const string Engine = "aimmod-osu-2026.730.0-v1";

    public static CalculationResult Calculate(CalculationRequest request, CancellationToken token) {
        byte[] bytes;
        try { bytes = Convert.FromBase64String(request.Map); }
        catch { throw new ArgumentException("Invalid beatmap file."); }
        if (bytes.Length == 0 || bytes.Length > 4 * 1024 * 1024 || !Convert.ToHexString(MD5.HashData(bytes)).Equals(request.Checksum, StringComparison.OrdinalIgnoreCase))
            throw new ArgumentException("The exact beatmap revision is unavailable.");
        bool lazer = request.Input?.Lazer ?? request.Lazer ?? true;
        if (request.Input is { } input && (input.Version != 1 || input.RulesetId != 0 || input.Lazer is null || input.Statistics is null || input.Statistics.Values.Any(n => n < 0) || input.MaxCombo < 0 || !double.IsFinite(input.Accuracy) || input.Accuracy < 0 || input.Accuracy > 1))
            throw new ArgumentException("Exact osu!standard score inputs are required.");
        if (!double.IsFinite(request.Accuracy) || request.Accuracy < 0 || request.Accuracy > 100) throw new ArgumentException("Accuracy must be between 0 and 100.");
        var ruleset = new OsuRuleset();
        var rawMods = request.Input?.Mods ?? request.Mods;
        if (rawMods.ValueKind != JsonValueKind.Array || rawMods.GetArrayLength() > 16) throw new ArgumentException("Invalid mods.");
        var apiMods = JsonConvert.DeserializeObject<APIMod[]>(rawMods.GetRawText())!;
        Mod[] mods = apiMods.Select(mod => mod.ToMod(ruleset)).ToArray();
        if (mods.Any(mod => mod is UnknownMod)) throw new ArgumentException("This mod is not supported by the calculator.");
        if (mods.Select(mod => mod.Acronym).Distinct().Count() != mods.Length || mods.Any(mod => mod.IncompatibleMods.Any(type => mods.Any(other => other != mod && type.IsInstanceOfType(other)))))
            throw new ArgumentException("The selected mods are incompatible.");
        if (!lazer && !mods.Any(mod => mod is ModClassic)) mods = [..mods, ruleset.CreateMod<ModClassic>()!];
        string path = Path.Combine(Path.GetTempPath(), $"aimmod-pp-{Guid.NewGuid():N}.osu");
        try {
            File.WriteAllBytes(path, bytes);
            var working = new FlatWorkingBeatmap(path);
            if (working.BeatmapInfo.Ruleset.OnlineID != 0) throw new ArgumentException("Only osu!standard maps are supported.");
            if (working.Beatmap.HitObjects.Count > 100000) throw new ArgumentException("The beatmap has too many objects.");
            var difficulty = ruleset.CreateDifficultyCalculator(working);
            var attributes = (OsuDifficultyAttributes)difficulty.Calculate(mods, token);
            int objectCount = attributes.HitCircleCount + attributes.SliderCount + attributes.SpinnerCount;
            if (objectCount <= 0 || objectCount > 100000) throw new ArgumentException("Invalid beatmap object count.");
            var stats = request.Input is { } actual ? ActualStatistics(actual) : GenerateStatistics(objectCount, request.Accuracy / 100);
            long judged = (long)stats.GetValueOrDefault(HitResult.Great) + stats.GetValueOrDefault(HitResult.Ok) + stats.GetValueOrDefault(HitResult.Meh) + stats.GetValueOrDefault(HitResult.Miss);
            if (judged > objectCount || (request.Input?.Passed != false && judged != objectCount)) throw new ArgumentException("Score judgements do not match this beatmap revision.");
            if (request.Input?.MaxCombo > attributes.MaxCombo) throw new ArgumentException("Score combo exceeds the beatmap maximum.");
            if (stats.GetValueOrDefault(HitResult.SliderTailHit) > attributes.SliderCount) throw new ArgumentException("Slider judgements do not match this beatmap.");
            if (request.Input is null) {
                stats[HitResult.SliderTailHit] = attributes.SliderCount;
                stats[HitResult.LargeTickMiss] = 0;
            }
            double accuracy = request.Input?.Accuracy ?? (stats[HitResult.Great] * 6d + stats[HitResult.Ok] * 2d + stats[HitResult.Meh]) / (objectCount * 6d);
            double Performance(Dictionary<HitResult,int> judgements, double acc, int combo, OsuDifficultyAttributes difficultyAttributes) {
                var score = new ScoreInfo(working.BeatmapInfo, ruleset.RulesetInfo) {
                    Mods = mods, IsLegacyScore = !lazer, Accuracy = acc, MaxCombo = combo, Statistics = judgements,
                    TotalScore = request.Input?.TotalScore ?? 0, LegacyTotalScore = request.Input?.LegacyTotalScore,
                    Passed = request.Input?.Passed ?? true,
                };
                return ruleset.CreatePerformanceCalculator().Calculate(score, difficultyAttributes).Total;
            }
            var scoreAttributes = attributes;
            if (request.Input?.Passed == false && judged > 0 && judged < objectCount) {
                scoreAttributes = difficulty.CalculateTimed(mods, token).Select(timed => (OsuDifficultyAttributes)timed.Attributes)
                    .Last(a => a.HitCircleCount + a.SliderCount + a.SpinnerCount <= judged);
            }
            double pp = judged == 0 ? 0 : Performance(stats, accuracy, request.Input?.MaxCombo ?? attributes.MaxCombo, scoreAttributes);
            var perfect = new Dictionary<HitResult,int> { [HitResult.Great] = objectCount, [HitResult.SliderTailHit] = attributes.SliderCount };
            double maxPp = Performance(perfect, 1, attributes.MaxCombo, attributes);
            if (!double.IsFinite(pp) || !double.IsFinite(maxPp) || pp < 0 || maxPp < 0) throw new ArgumentException("Calculation returned invalid PP.");
            return new(pp, maxPp, attributes.StarRating, objectCount, accuracy, lazer, Engine, difficulty.Version);
        } finally { File.Delete(path); }
    }

    static Dictionary<HitResult,int> ActualStatistics(ScoreInput input) {
        var result = new Dictionary<HitResult,int>();
        foreach (var (key, count) in input.Statistics!) {
            if (Enum.TryParse<HitResult>(key.Replace("_", ""), true, out var hit)) result[hit] = count;
        }
        return result;
    }

    // Same minimum-error judgement search as AimMod's native PP worker.
    public static Dictionary<HitResult,int> GenerateStatistics(int objects, double target) {
        int great = objects, ok = 0, meh = 0;
        double bestError = Math.Abs(1 - target);
        for (int n50 = 0; n50 <= objects; n50++) {
            int rounded = Math.Clamp((int)Math.Round((6d * objects - 5d * n50 - target * objects * 6) / 4), 0, objects - n50);
            for (int n100 = Math.Max(0, rounded - 1); n100 <= Math.Min(objects - n50, rounded + 1); n100++) {
                int n300 = objects - n100 - n50;
                double error = Math.Abs((6d * n300 + 2d * n100 + n50) / (6d * objects) - target);
                if (error < bestError) { great = n300; ok = n100; meh = n50; bestError = error; }
            }
        }
        return new() { [HitResult.Great] = great, [HitResult.Ok] = ok, [HitResult.Meh] = meh, [HitResult.Miss] = 0 };
    }
}
