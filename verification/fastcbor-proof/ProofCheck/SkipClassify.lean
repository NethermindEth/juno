import ProofCheck.SkipClassificationSupport

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Definite byte/text bodies either supply exactly the declared bytes or have
    the specified truncation error. Ignored text has no UTF-8 requirement. -/
theorem skip_bytes_classify (d : Nat) (head : w8) (payload rest : List w8) (argument : w64)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 64 ∨ head &&& W8 224 = W8 96) :
    SkipClassified d ((head :: payload) ++ rest) := by
  by_cases short : rest.length < uint.Z argument
  · exact Or.inr ⟨1 + payload.length, .atomic .truncated,
      skipFailure_here _ _ _ _ (.atomic enc (.bytes hm short))⟩
  have hn : 0 ≤ uint.Z argument := by word
  have hsize : (uint.Z argument).toNat ≤ rest.length := by omega
  have hlen : uint.Z argument = (rest.take (uint.Z argument).toNat).length := by
    simp only [List.length_take, Nat.min_eq_left hsize]; omega
  refine Or.inl ⟨(head :: payload) ++ rest.take (uint.Z argument).toNat, rest.drop (uint.Z argument).toNat, ?_, ?_⟩
  · rw [List.append_assoc, List.take_append_drop]
  · exact .atom enc (.bytes hm _ hlen)

/-- Complete simple heads have precisely the preferred-width restriction in
    the ignored-value contract; all other widths/values are accepted. -/
theorem skip_simple_classify (d : Nat) (head : w8) (payload rest : List w8) (argument : w64)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 224) :
    SkipClassified d ((head :: payload) ++ rest) := by
  by_cases nonpreferred : head &&& W8 31 = W8 24 ∧ uint.Z argument < 32
  · exact Or.inr ⟨1 + payload.length, .atomic .simple,
      skipFailure_here _ _ _ _ (.atomic enc (.simple hm nonpreferred.1 nonpreferred.2))⟩
  have preferred : head &&& W8 31 ≠ W8 24 ∨ 32 ≤ uint.Z argument := by
    by_cases hwidth : head &&& W8 31 = W8 24
    · exact Or.inr (by by_cases hsmall : uint.Z argument < 32; exact False.elim (nonpreferred ⟨hwidth, hsmall⟩); omega)
    · exact Or.inl hwidth
  exact Or.inl ⟨head :: payload, rest, rfl, by simpa only [List.append_nil] using SkipValid.atom enc (SkipAtom.simple hm preferred)⟩

/-- Every finite byte sequence at a supported nesting depth either begins with
    an independently specified valid item or has a specified rejection. All
    recursive child classifications are discharged on strictly shorter input. -/
theorem skip_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) : SkipClassified d wire := by
  cases wire with
  | nil => exact Or.inr ⟨0, .eof, skipFailure_here _ _ _ _ (.eof d)⟩
  | cons head tail =>
    cases head_classify head tail with
    | fault reason fault => exact Or.inr ⟨0, .head head reason, skipFailure_here _ _ _ _ (.head tail fault)⟩
    | truncated size info sizeEq short => exact Or.inr ⟨0, .incomplete,
        skipFailure_here _ _ _ _ (.incomplete tail info sizeEq short)⟩
    | encoded payload rest argument split enc =>
      subst tail
      rcases byte_major_cases head with hm | hm | hm | hm | hm | hm | hm | hm
      · exact Or.inl ⟨head :: payload, rest, rfl,
          by simpa only [List.append_nil] using SkipValid.atom enc (SkipAtom.integer (Or.inl hm))⟩
      · exact Or.inl ⟨head :: payload, rest, rfl,
          by simpa only [List.append_nil] using SkipValid.atom enc (SkipAtom.integer (Or.inr hm))⟩
      · exact skip_bytes_classify d head payload rest argument enc (Or.inl hm)
      · exact skip_bytes_classify d head payload rest argument enc (Or.inr hm)
      · apply skip_container_classify d head payload rest argument (W64 1) enc (Or.inl ⟨hm, rfl⟩) hdepth
        intro bound child hlen
        exact skip_classify (d + 1) child (by omega)
      · apply skip_container_classify d head payload rest argument (W64 2) enc (Or.inr ⟨hm, rfl⟩) hdepth
        intro bound child hlen
        exact skip_classify (d + 1) child (by omega)
      · by_cases small : uint.Z argument ≤ 3
        · exact Or.inr ⟨1 + payload.length, .atomic .tag,
            skipFailure_here _ _ _ _ (.atomic enc (.tag hm small))⟩
        have number : 3 < uint.Z argument := by omega
        cases rest with
        | nil =>
          exact Or.inr ⟨1 + payload.length, .eof,
            by simpa only [List.append_nil] using skipFailure_here _ _ _ _ (SkipFailureLeaf.tagEOF enc hm number)⟩
        | cons next rest =>
          by_cases tooDeep : next &&& W8 224 = W8 192 ∧ d = 32
          · obtain ⟨ht, rfl⟩ := tooDeep
            exact Or.inr ⟨1 + payload.length, .depth, skipFailure_here _ _ _ _ (.tagDepth enc hm number ht)⟩
          have bound : next &&& W8 224 = W8 192 → d < 32 := by
            intro ht
            by_cases hd : d = 32
            · exact False.elim (tooDeep ⟨ht, hd⟩)
            · omega
          have hc : (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) ≤ 32 := by
            by_cases ht : next &&& W8 224 = W8 192
            · have := bound ht; simp only [ht, ite_true]; omega
            · simp only [ht, ite_false, Nat.add_zero]; exact hdepth
          rcases skip_classify (d + if next &&& W8 224 = W8 192 then 1 else 0) (next :: rest) hc with
            ⟨item, suffix, split, valid⟩ | ⟨offset, reason, failure⟩
          · have hn := skipValid_nonempty _ item valid
            cases item with
            | nil => simp only [List.length_nil] at hn; omega
            | cons first body =>
              have hfirst : next = first := by simpa using congrArg List.head? split
              subst first
              refine Or.inl ⟨(head :: payload) ++ (next :: body), suffix, ?_, .tag enc hm number bound valid⟩
              rw [List.append_assoc, ← split]
              rfl
          · exact Or.inr ⟨1 + payload.length + offset, reason,
              skipFailure_tag d offset head next payload rest argument reason enc hm number bound failure⟩
      · exact skip_simple_classify d head payload rest argument enc hm
termination_by wire.length
decreasing_by
  all_goals simp_wf
  all_goals subst_vars
  all_goals simp only [List.length_cons, List.length_append] at *
  all_goals omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_bytes_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_simple_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_classify
