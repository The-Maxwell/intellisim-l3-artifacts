# EXP-H scoring - report 12

Meta: sim_id=6a5bbcc1c44d001ef3acc10e analysis_id=6a63a4fe4ccaa6983ac98334 lang=uk llm_target=o3 generated=2026-07-24T17:46:38.068Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-2.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди евакуюються з одного приміщення через одні двері. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o3"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 2 | "Середня продуктивність дверей від першого до останнього виходу: 100 людей за 41 крок ≈2,4 особи/крок." ... "Трьохклітинний вихід обмежив пропускну здатність, тому середній темп становив ≈2.2 особи/крок (100 / 45)." ... "щонайменше 28 осіб залишалися у тонкому поясі x = 33–40, y = 18–22, тобто безпосередньо перед трьохклітинними східними дверима. Це свідчить про виникнення щільного «корка»" (sheet: ~2.3 agents/step, range 2.0-2.5; door queue - within range) |
| 2 | 2 | "Жодних коливань чи повернень назад не спостерігалось – чисельність зменшувалась монотонно." ... "Самоорганізована стійка черга перед дверима, що підтримує квазілінійний потік евакуації." ... "головним обмеженням є пропускна здатність дверей, а не індивідуальна швидкість" (sheet: monotone near-linear, flow-limited - explicitly identified; overall rate 2.2-2.4/step within range) |
| 3 | 2 | "Повна евакуація завершилась на кроку 45; тривалість процесу ≈13,5 с." ... "Повна евакуація 100 осіб зайняла 44 такти (перший вихід на кроці 4, останній — між 44 і 45)." (sheet: 44 steps = 13.2 s; ensemble 13.2-14.1 s / 44-47 steps - within range; no ~110 s claim) |

**Phenomenon points: 6/6**

Factual errors (verbatim + what the data says): count = 2
- 'між кроками 8 і 34 крізь двері проходить від 2 до 4 осіб за такт, у середньому близько 2,9' / 'Пропускна здатність дверей зафіксована на рівні 2–4 особи/крок (середньо 2,4–2,9)' - the data-supported door throughput is ~2.3 agents/step (range 2.0-2.5) with a near-linear discharge; a sustained 2.9/step over 27 of 44 steps is outside the range and inconsistent with near-linearity.
- 'Ширина дверей 1,2 м обмежила миттєву пропускну здатність → плато 4–5 осіб/крок' - a plateau of 4-5 agents/step is roughly double the data-supported cap of ~2.3 agents/step (range 2.0-2.5); it also contradicts the report's own "2–4 особи/крок" figure.

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Фазовий перехід «черга → розріджений хвіст» після зменшення натовпу, який знижує середній відтік удвічі.' (also 'Коли у приміщенні залишилось <15 осіб (після кроку 34), потік впав до 1 особи/крок, подовживши «хвіст» евакуації на 11 тактів.') - the data shows a monotone near-linear evacuated_count 0->100 over 44 steps; an asserted regime change that halves the outflow for the last quarter of the run is not supported by the data (a 1/step trickle is evident only for the final steps: 98@42, 99@43, 100@44).

Actionable recommendations: y - quote: "Перезапустити модель із ширшими дверима (≥2,4 м, 6 клітин) або кількома отворами, щоб кількісно оцінити скорочення часу і зникнення \"довгого хвоста\"."

Language/format notes: Ukrainian Cyrillic renders correctly in JSON; one Russian-letter typo "эвакуації" in the spatial narrative; unusual coinage "Екватор евакуації". All sections, executive summary, 6 conclusions and 6 recommendations present. Internal wobbles (not counted as errors): total time given as "45 кроків / ≈13,5 с" in dynamics/spatial/summary/conclusions vs "44 такти" in the emergence section (both inside the 44-47 / 13.2-14.1 s range); "97 % усіх виходів (кроки 4–34)" is inconsistent with the report's own 1-per-step tail over steps 38-45; "дворядна черга" (two-row queue) is a specific claim the sheet neither confirms nor refutes.
