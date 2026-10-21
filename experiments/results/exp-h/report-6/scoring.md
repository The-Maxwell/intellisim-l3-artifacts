# EXP-H scoring - report 6

Meta: sim_id=6a63810f4ccaa6983ac81367 analysis_id=6a63a39f4ccaa6983ac98322 lang=uk llm_target=o4-mini generated=2026-07-24T17:40:47.515Z scorer=Claude (author) scored=2026-07-24
Ground-truth sheet: `results/exp-h/ground-truth/sheet-6.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з приміщення через одні двері; частину рішень агентів під час виконання ухвалює мовна модель. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "50 % агентiв евакуйованi до кроку 22; 90 % - до кроку 38 ... остання особа евакуювалася ... до кроку 45" |
| 2 | 1 | "правило decide_proceed_to_exit виконувалося 494 рази" |
| 3 | 2 | "до 45-го кроку останнiй агент залишив кiмнату ... модель завершилася на 46-му кроцi" |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- exec summary: 'через одноклiтинний дверний отвiр' - single-cell door; the door is 3 cells (the emergence section correctly says 'триклiтинний')

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Розширити дверний отвiр або додати другий вихiд для зменшення черг"

Language/format notes: uk rendered correctly. P1=2: egress ~45-46 steps captured and NOT wrongly claimed slower than baseline (the GT trap avoided). P2=1: reports proceed_to_exit=494 (correct, degenerate) but does not explicitly flag the all-proceed/no-hesitation degeneracy as a finding. P3=2: full evacuation affirmed.
