# EXP-H scoring - report 6

Meta: sim_id=6a63810f4ccaa6983ac81367 analysis_id=6a63a39f4ccaa6983ac98322 lang=uk llm_target=o4-mini generated=2026-07-24T17:40:47.515Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-6.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з приміщення через одні двері; частину рішень агентів під час виконання ухвалює мовна модель. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 1 | "Усі 100 агентів евакуйовані до кроку 46" + "До 45-го кроку останній агент залишив кімнату, що демонструє відсутність тривалого затору." (T = 46 steps correct and no prolonged jam noted, but the report never identifies that egress equals the rules-only optimum / that the LLM decisions had no slowing effect; it instead suggests the decision rule caused pauses) |
| 2 | 1 | "Rule «decide_proceed_to_exit» виконано 494 рази, «exit_when_at_door» – 100 разів." (494 proceed_to_exit executions correct, but the degenerate mix - no wait_briefly / follow_crowd / seek_alternate_route at all - is never explicitly noted as such) |
| 3 | 2 | "Усі 100 агентів евакуйовані до кроку 46" + "«exit_when_at_door» спрацьовувало 100 разів, вилучаючи кожного агента без можливості повторного доступу до кімнати" + "Фінальний агент вийшов до 45-го кроку; модель завершилася на 46-му кроці." (full egress, run ends at step 46 on population -> 0) |

**Phenomenon points: 4/6**

Factual errors (verbatim + what the data says): count = 0
- (none)

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Висока частота виконань правила decide_proceed_to_exit (494 рази) сприяла коротким паузам у русі агентів, що проявилося в плато на кроках 2–3 і 6–7.' (repeated in conclusions as 'Початкова фаза евакуації уповільнена плато на кроках 2–3 через часті рішення agents щодо продовження руху («decide_proceed_to_exit»)' and turned into the recommendation 'Зменшити частоту виконання правила «decide_proceed_to_exit»') - no data support for LLM decision executions causing movement pauses: the run's egress (46 steps, 6.76 +/- 3.25 s) is indistinguishable from the rules-only baseline, so the decision rule had no measurable slowing effect; the asserted mechanism is invented.

Actionable recommendations: y - quote: "Розширити дверний отвір або додати другий вихід для зменшення черг та прискорення завершальної фази."

Language/format notes: Predominantly Ukrainian (Cyrillic), but the third section title is left in English ("Emergent Behavior & Causal Relationships"), and Latin words leak into Ukrainian sentences ("Rule «decide_proceed_to_exit»", "рішення agents", "agent-agent", "cell/step"). Internal wobbles (not counted): door width given as "одноклітинний дверний отвір" in the executive summary vs "триклітинний дверний отвір" in the emergence section; last exit given as "до кроку 45" vs "до кроку 46"; peak-rate interval "кроки 17–23" vs "кроки 4–17 ... до 4–5 агентів за крок"; "втрата ~29 осіб" for 97->66 (=31). Step-level plateaus (2–3, 6–7, 44), median/90% steps and total distance ~2578.8 are not verifiable from the sheet and are treated as unknown.
