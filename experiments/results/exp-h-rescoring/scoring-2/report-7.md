# EXP-H scoring - report 7

Meta: sim_id=6a610ff2ff82d466bf2db995 analysis_id=6a63a3d14ccaa6983ac98325 lang=uk llm_target=o4-mini generated=2026-07-24T17:41:37.046Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-7.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Вогонь поширюється лісом. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "До завершення симуляції (крок 10) горіння зійшло нанівець, натомість попелу накопичилося 138 одиниць." ... "Закінчення горіння до кроку 10, коли кількість горючих дерев стала нульовою." |
| 2 | 2 | "Лише 7.66% дерев (138 із 1802) були знищені, що свідчить про недостатню зв’язність лісової маси при початковій щільності ≈0.5." ... "незгорілих залишилося 1723 із початкових 1802 – це падіння на 4,4 %" |
| 3 | 1 | "Лише 7.66% дерев були знищені, пожежа не охопила весь ліс." ... "Самозупинка пожежі через фрагментацію лісу: пожежа не перетворюється на суцільне полум’я" ... "Самообмеженість поширення вогню через перервану коннективність підпорогової густини дерев." |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'Фронтова хвиля горіння рухається справа наліво й поступово згасає за відсутності нової деревини за фронтом.' - the run ignites the left edge (the report itself states ignite_left_edge at step 0) and the data's percolation check is "no ash in the rightmost column x=60", i.e. the far edge is the right one; the front moves left-to-right, not right-to-left.

Hallucinated phenomena (verbatim + why unsupported): count = 0
- (none)

Actionable recommendations: y - quote: "Провести серії симуляцій із різними щільностями дерев (наприклад 0.3–0.7) для виявлення порогу перколяції."

Language/format notes: Report is in Cyrillic Ukrainian as requested (some untranslated English tokens: "unburned", "ash", "snapshot Step 0", "burning_tree.peak"; typos/non-standard words: "зоніями", "гілля", "попалених", "коннективність", "транзіція"). Phenomenon 3 scored 1: the report never states that the fire failed to reach the far (right) edge; non-percolation is only implied via "не охопила весь ліс" and "підпорогової густини". Internal wobbles (not counted as errors, all burned-fraction figures fall within the 0.04-0.15 range): burned share given as "4,4 %" (79/1802), "приблизно 8 % від початкової популяції unburned trees (1723)" (1723 is the final, not initial, count) and "7.66%" (138/1802); spread_fire 79 executions placed "між кроками 1–9" in one place and "до кроку 2" in another; "Протягом перших 11 кроків вогонь послідовно ініціювався на лівому краї" contradicts the report's own statement that ignition happened only at step 0; "Лінійне накопичення попелу: +7–20 одиниць щокроку" contradicts its own ash jump 0→59 at step 1. Claims about toroidality/absence of periodic boundaries and the secondary burning peak (20 at step 2) are not verifiable from the sheet and were treated as unknown.
