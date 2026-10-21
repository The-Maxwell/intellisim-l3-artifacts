# EXP-H scoring - report 2

Meta: sim_id=6a5bbcc1c44d001ef3acc10e analysis_id=6a63a2a44ccaa6983ac98316 lang=uk llm_target=o4-mini generated=2026-07-24T17:36:36.53Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-2.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з одного приміщення через одні двері. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "100 агентів евакуйовані за 45 кроків (середній потік ~2.22 агента/крок)." ... "Вузький отвір (3 клітини) спричинив затори та чергування біля дверей." ... "Спонтанна черга перед дверима як результат індивідуального руху та блокувань." (door queue explicit; 2.22 agents/step inside the 2.0–2.5 range) |
| 2 | 1 | "Самоорганізований стабільний потік ≈3 особи/крок у центральній фазі без централізованого регулювання руху." ... "протягом кроків 13–25 пропускна здатність стабілізувалась на рівні ~2–3 осіб/крок" (a stable self-organised flow is identified, but the rate ≈3 is above the 2.0–2.5 range and the report frames the run as accelerate–plateau–slow down rather than the near-linear discharge the data show) |
| 3 | 1 | "Останній агент вийшов близько кроку 43, отже загальна тривалість евакуації склала приблизно 43 кроки (~57 с)." ... "Всі 100 агентів евакуйовані до кроку 45" (total egress identified in steps, 43/45 vs. measured 44 - imprecise; the conversion to ~57 s contradicts the measured 13.2 s and is counted as a factual error below) |

**Phenomenon points: 4/6**

Factual errors (verbatim + what the data says): count = 2
- 'Загальний час евакуації — ≈43 кроки (~57 с).' (also "остаточна евакуація завершилась на кроці 43 (≈57 с)") - measured T_total = 13.2 s = 44 steps at 0.3 s/step (ensemble 13.2–14.1 s, 44–47 steps); ~57 s is ~4.3x the measured egress time and 43 steps is just below the data-supported range
- 'Стабільна середня швидкість −≈3 особи/крок спостерігалась від кроку 9 до 30.' (also "середня швидкість виходу утримувалася на рівні приблизно 3 осіб за крок") - the data show ~2.3 agents/step (data-supported 2.0–2.5) with a near-linear discharge; a sustained ≈3/step over half the run is outside the range and inconsistent with the report's own overall average of 2.22

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Фінальна фаза евакуації (step 34–43) сповільнилася до ~1 особи/крок.' (also "Локальні пробки та нестабільний потік у кінцевій фазі евакуації" and "зіткнення спричинили зниження пропускної здатності у фінальних кроках") - the evacuated_count rises near-linearly to the end (98@42, 99@43, 100@44); no end-phase slowdown to ~1/step or collision-driven "local jams" is supported by the data

Actionable recommendations: y - quote: "Провести серію прогонів із різними ширинами виходу (4–6 клітин) для оцінки впливу отвора на пропускну здатність."

Language/format notes: Ukrainian throughout as requested; Cyrillic renders correctly in the JSON (no tofu). All three sections plus executive summary, conclusions and recommendations present. Minor wording defects: "стіно­колізій" contains a stray soft hyphen; "затримання"/"розбігання" are awkward word choices; the dynamics title leaks the internal experiment name "exp-evac-V2-reference". Internal wobbles (notes): last exit given as step 43 in dynamics/emergence/summary but "до кроку 45" in the spatial section; "Зменшення чисельності групи нижче ≈15 осіб після кроку 34" is not checkable against the sheet. PDF not part of the blind set, not checked.
