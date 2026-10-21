# EXP-H scoring - report 10

Meta: sim_id=6a636c0c4ccaa6983ab1976a analysis_id=6a63a46f4ccaa6983ac9832e lang=uk llm_target=o4-mini generated=2026-07-24T17:44:15.641Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-10.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Інфекційне захворювання поширюється в популяції рухомих агентів. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Пікова кількість інфікованих 20 осіб за кроки 50–52." ... "Модель продемонструвала класичну однопікову епідемічну криву" ... "Епідемічна хвиля з одноразовим піком заражень (growth–peak–decline)." (sheet: peak 21 at step 51; range height ~17-25, timing ~40-60 - within range) |
| 2 | 2 | "До кінця симуляції (крок 173) усі інфіковані (66 осіб за весь експеримент) одужали, сприйнятливих залишилося 134." (sheet: final_recovered 66/200 = 0.33; range ~0.28-0.38 - within range) |
| 3 | 1 | "Одужалих агентів наростало з 0 (крок 0) до 66 (крок 173), одночасно сприйнятливих зменшилося з 197 до 134." (direction and endpoints of S and R correct, but monotonicity/no-reversal is never stated explicitly - qualitative identification) |

**Phenomenon points: 5/6**

Factual errors (verbatim + what the data says): count = 1
- 'До середини симуляції (Step 86) скупчення не сформувалися: позиції 66 поточних піддатливих (susceptible) агентів продовжують охоплювати весь простір' - the data has S non-increasing and ending at 134 (200 - 66 recovered), so S at step 86 must be >= 134, not 66; 66 is the final recovered count, misattributed to susceptibles.

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Далі система зазнала кількох менших хвиль (кроки 90–100) із поверненням інфікованих до 9' / 'Вторинні сплески інфекції спостерігалися на кроках 56–64 та 90–100' / 'Хвилеподібна епідемічна крива з початковим піком та затухаючими вторинними сплесками.' - the sheet's data shows a single hump (one peak at step 51, then the epidemic ends); asserted secondary waves / rebound to 9 infected are not supported by the data.

Actionable recommendations: y - quote: "Провести чутливісний аналіз швидкості руху агентів для оцінки впливу зниження мобільності на утворення локальних вогнищ."

Language/format notes: Ukrainian Cyrillic renders correctly; one section title is mixed English/Ukrainian ("Emergent Behavior & Causal Relationships: Просторова SIR-модель"); the emergence narrative contains stray control characters () around "infection_timer". Internal wobbles (not counted as errors): total ever-infected given as "64" in key_findings/conclusions vs "66" in narrative (64/200 = 0.32 is still inside the 0.28-0.38 range); peak given as 20 (dynamics/emergence) vs "peak 21" (spatial) - both inside range. All sections, executive summary, conclusions and 6 recommendations present. Report does not compare final size to the well-mixed analytic bound (not required for the score).
