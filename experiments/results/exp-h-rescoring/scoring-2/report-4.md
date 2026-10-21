# EXP-H scoring - report 4

Meta: sim_id=6a5bc463c44d001ef3b8f824 analysis_id=6a63a3444ccaa6983ac9831c lang=uk llm_target=o4-mini generated=2026-07-24T17:39:16.482Z scorer=pass-2-blind scored=2026-09-08
Ground-truth sheet: `results/exp-h/ground-truth/sheet-4.md` (committed at 23ff33d, BEFORE generation)
Request body: {"context_message": "Проаналізуйте результати цієї симуляції. Контекст: Люди проходять крізь широкий отвір за різної щільності натовпу. Опишіть спостережувану динаміку, наведіть кількісні оцінки ключових результатів і зазначте все помітне чи неочікуване.", "language": "uk", "llm_target": {"provider_uid": "openai", "model_uid": "o4-mini"}, "selection": {}}

| # | Score 0/1/2 | Evidence (verbatim report quote) |
|---|---|---|
| 1 | 1 | "Загалом 297 осіб покинули приміщення за перші 90 кроків, тоді як далі процес практично зупинився." (rapid, near-complete discharge identified; but the flow is characterized as phased "~2.9 за крок" / "~6.5 за крок" and the only stated average is "середній потік ≈0,66 агента/крок" - no sustained rate within 3.3-3.9 agents/step is given) |
| 2 | 1 | "Подальше падіння чисельності нижче ~63 осіб на кроці 45 → перехід у вільний режим руху й зниження потоку (44 особи за 15 кроків)" (flow falling with density is implied on the low branch only; the report frames the relation as non-monotone: "Нелінійна залежність пропускної здатності від щільності: максимальний потік спостерігається при середній концентрації (~160 осіб)") |
| 3 | 0 | "Висока початкова щільність (>300 агентів) → утворення заторів біля отвору та низький потік (43 особи за 15 кроків)" (asserts a classic high-density congestion slowdown; data show no saturation - Fs keeps rising beyond ~2 P/m^2) |

**Phenomenon points: 2/6**

Factual errors (verbatim + what the data says): count = 2
- 'Висока початкова щільність (>300 агентів) → утворення заторів біля отвору та низький потік (43 особи за 15 кроків)' (repeated as 'початкова фаза заторів (0–15 кроків)' and 'Нелінійна залежність пропускної здатності від щільності: максимальний потік спостерігається при середній концентрації (~160 осіб)') - data: specific flow rises monotonically with density and keeps rising ~linearly beyond ~2 P/m^2 (Fs 3.0-5.7 at D 3-6); there is no high-density plateau or congestion slowdown. The lower exit count in steps 0-15 is consistent with agents still walking to the opening, not a jam.
- 'Вихід 297 агентів через отвір завершився до кроку 449, середній потік ≈0,66 агента/крок.' - data: 297/300 agents exit by step 83, i.e. ~3.6 agents/step sustained (data-supported 3.3-3.9); 0.66 divides the exits by an interval ~5x longer than the actual discharge and misstates the throughput.

Hallucinated phenomena (verbatim + why unsupported): count = 1
- 'Обмежена ширина отвору (10 клітин) → формування стабільного затору з 3 агентів до кінця симуляції.' (also 'Самоорганізована черга, що встановила пропускну межу системи, незалежно від подальших дій окремих агентів.' and 'Підвищена локальна щільність біля отвору (до 3 агентів на суміжних клітинах x=39–42) → уповільнення руху → формування стабільної черги.') - the 3 stragglers that carry the run to max_steps 899 are not a queue/clog caused by opening width: a 10-cell (4 m) opening cannot be blocked by 3 agents, the data show no capacity limit at any density, and nothing in the data supports a self-organized jam that "sets the system's throughput limit".

Actionable recommendations: y - quote: "Провести серію симуляцій зі збільшеною шириною отвору (наприклад 12–15 клітин) для перевірки зниження залишкового затору."

Language/format notes: Ukrainian (Cyrillic) throughout, all three sections + executive summary + conclusions + recommendations present. Minor artifacts: Latin "i" inside Cyrillic words in the executive summary ("камерi", "стабiльна"); grammatical slip "залишающимися". Internal wobbles (not counted): phase boundaries inconsistent ("швидкий відтік із 0 по 45 крок" vs "швидкий відтік (0–15)"); "297 ... за перші 90 кроків" vs "завершився до кроку 449"; spatial section reports only a 50-agent sample at step 0 ("50 випадково вибраних агентів") without noting the population is 300. Phase counts (300→257→160→63→19) are consistent with the sheet's 20-step trajectory (300→223→92→19→4).
