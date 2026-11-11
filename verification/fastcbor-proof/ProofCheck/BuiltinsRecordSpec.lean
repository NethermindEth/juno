import ProofCheck.BuiltinsPairs
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A direct BuiltinInstanceCounter record preserves old contents on null/undefined. Successful
    map decoding retains the exact unread suffix. -/
inductive BuiltinsRecordValid (d : Nat) (old : BuiltinsValue) : List w8 → List w8 → BuiltinsValue → Prop where
  | nullish {head : w8} (suffix : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      BuiltinsRecordValid d old [head] suffix old
  | map {head payload body suffix argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32)
      (valid : BuiltinsPairsValid (d + 1) count [] old body suffix final) :
      BuiltinsRecordValid d old ((head :: payload) ++ body) suffix final

/-- Opening errors consume nothing and preserve depth. Later failures retain
    the partial record and one entered map level; there is no record rollback. -/
inductive BuiltinsRecordRejected (d : Nat) (old : BuiltinsValue) : List w8 → BuiltinsValue →
    Nat → Nat → Bool → BuiltinsFailureReason → Prop where
  | opening {wire reason} (failure : StructOpenFailure d wire reason) :
      BuiltinsRecordRejected d old wire old 0 0 false (.opening reason)
  | map {head payload body argument final consumed offset reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : BuiltinsPairsRejected (d + 1) count [] old body final consumed offset reason) :
      BuiltinsRecordRejected d old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) true reason

/-- All inputs are classified, including arbitrary field orders, unknown keys,
    duplicate known keys and failures after any successful prefix. -/
theorem builtinsRecord_classify (d : Nat) (old : BuiltinsValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ BuiltinsRecordValid d old item rest final) ∨
    (∃ final consumed offset entered reason, BuiltinsRecordRejected d old wire final consumed offset entered reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish tail nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases builtinsPairs_classify (d + 1) count [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major size hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, true, reason,
        .map enc major size hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, false, _, .opening failure⟩

theorem builtinsRecord_nonempty (d : Nat) (old final : BuiltinsValue) (item rest : List w8)
    (valid : BuiltinsRecordValid d old item rest final) : 0 < item.length := by
  cases valid <;> simp

theorem builtinsRecordRejected_bounds (d consumed offset : Nat) (old final : BuiltinsValue) (wire : List w8)
    (entered : Bool) (reason : BuiltinsFailureReason)
    (failure : BuiltinsRecordRejected d old wire final consumed offset entered reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | opening => omega
  | map enc major size depth fit failure =>
    have hb := builtinsPairsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecord_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecord_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecordRejected_bounds
