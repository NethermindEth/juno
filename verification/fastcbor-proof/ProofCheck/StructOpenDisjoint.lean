import ProofCheck.HeaderDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Opening failures cannot override the record reader's null/undefined check. -/
theorem structOpenFailure_not_nullish (d : Nat) (wire : List w8) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | outer hn hu failure => exact ⟨hn, hu⟩
  | count enc major short =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | depth enc major fit =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction

/-- A complete map head within the depth limit and fitting the unread bytes
    cannot have an opening error; exact head-width uniqueness protects this
    precedence argument even for non-preferred argument widths. -/
theorem structOpenFailure_excludes_map (d : Nat) (head : w8) (payload body : List w8)
    (argument : w64) (reason : FeltFailureReason)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) (fit : 2 * uint.Z argument ≤ body.length)
    (failure : StructOpenFailure d ((head :: payload) ++ body) reason) : False := by
  generalize eqWire : (head :: payload) ++ body = wire at failure
  cases failure with
  | outer hn hu failure =>
    exact expectedFailure_excludes_encoded _ head payload body wire argument _ enc major eqWire.symm failure
  | @count d other payload2 tail argument2 enc2 major2 short =>
    simp only [List.cons_append, List.cons.injEq] at eqWire
    obtain ⟨rfl, same⟩ := eqWire
    obtain ⟨payloadEq, argumentEq, bodyEq⟩ := headArgument_prefix_unique head payload payload2 body tail
      argument argument2 enc enc2 same
    subst argument2
    subst tail
    omega
  | depth enc2 major2 fit2 => omega

/-- No successful complete Header prefix can also fail its opening checks. -/
theorem headerRecordValid_not_open_failure (d : Nat) (initial final : HeaderValue)
    (item rest : List w8) (reason : FeltFailureReason)
    (valid : HeaderRecordValid d initial item final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := headerPairs_count_bound _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

/-- The same opening exclusion applies to complete nested GasPrice records. -/
theorem gasRecordValid_not_open_failure (d : Nat) (initial final : GasPriceValue)
    (item rest : List w8) (reason : FeltFailureReason)
    (valid : GasRecordValid d initial item final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := gasPairs_count_bound _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.structOpenFailure_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.structOpenFailure_excludes_map
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecordValid_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordValid_not_open_failure
