import ProofCheck.ProjectionTagRead

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

def projectionTagResult (unsupported : GoInterface) (tag : w64) (rest : List w8) : GoInterface := by
  classical
  exact if projectionTagRejected tag rest then unsupported else interface.nil

/-- Every successfully read tag head is consumed exactly once. The prelude
    returns a bare unsupported sentinel for reserved numbers or a following tag;
    EOF after an allowed tag is intentionally deferred to the record reader. -/
theorem wp_projectionTag_checked (c : Loc) (v : cursor.Cursor) (xs payload rest : List w8)
    (head : w8) (tag : w64) (dq : DFrac) (truncated unsupported : GoInterface)
    (enc : HeadArgument head payload tag) (major : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ truncated ∗ globalAddr cursor.ErrUnsupported ↦□ unsupported ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ RET #(projectionTagResult unsupported tag rest);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨#Htruncated, #Hunsupported, Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs (payload ++ rest) head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_tag_wire c v xs payload rest head tag dq enc major hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  cases rest with
  | nil =>
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length Hnext.2.2
    simp only [subslice_length (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' + W64 (1 + payload.length) = v.buf'.len := by
      simp only [sint.nat] at Hlen hlen; word
    wp_apply wp_peek_eof c {v with pos' := v.pos' + W64 (1 + payload.length)} truncated heof
      $$ [Hc] as %discarded ⟨Hc, Hdiscarded⟩
    · iframe Htruncated Hc
    by_cases reserved : uint.Z tag ≤ 3
    · have hcheck : uint.Z tag ≤ uint.Z (W64 3) := reserved
      simp only [hcheck, _root_.decide_true]
      wp_auto
      have resultEq : projectionTagResult unsupported tag [] = unsupported := by
        simp [projectionTagResult, projectionTagRejected, reserved]
      isimp only [resultEq] at HΦ
      iapply HΦ
      iframe Hc Hs
      ipureintro; exact Hnext
    · have hcheck : ¬ uint.Z tag ≤ uint.Z (W64 3) := reserved
      simp only [hcheck, _root_.decide_false]
      wp_auto
      have resultEq : projectionTagResult unsupported tag [] = interface.nil := by
        simp [projectionTagResult, projectionTagRejected, reserved, projectionHasTag]
      isimp only [resultEq] at HΦ
      iapply HΦ
      iframe Hc Hs
      ipureintro; exact Hnext
  | cons next tail =>
    wp_apply wp_peek_prefix c {v with pos' := v.pos' + W64 (1 + payload.length)} xs tail next dq
      ⟨by rw [Hnext.1]; omega, Hnext.2.1⟩ Hnext.2.2
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    by_cases reserved : uint.Z tag ≤ 3
    · have hcheck : uint.Z tag ≤ uint.Z (W64 3) := reserved
      simp only [hcheck, _root_.decide_true]
      wp_auto
      have resultEq : projectionTagResult unsupported tag (next :: tail) = unsupported := by
        simp [projectionTagResult, projectionTagRejected, reserved]
      isimp only [resultEq] at HΦ
      iapply HΦ
      iframe Hc Hs
      ipureintro; exact Hnext
    · have hcheck : ¬ uint.Z tag ≤ uint.Z (W64 3) := reserved
      simp only [hcheck, _root_.decide_false]
      wp_auto
      by_cases chained : next &&& W8 224 = W8 192
      · simp only [chained, _root_.decide_true]
        wp_auto
        have resultEq : projectionTagResult unsupported tag (next :: tail) = unsupported := by
          have rejected : projectionTagRejected tag (next :: tail) := Or.inr ⟨next, tail, rfl, chained⟩
          simp only [projectionTagResult, if_pos rejected]
        isimp only [resultEq] at HΦ
        iapply HΦ
        iframe Hc Hs
        ipureintro; exact Hnext
      · simp only [chained, _root_.decide_false]
        wp_auto
        have resultEq : projectionTagResult unsupported tag (next :: tail) = interface.nil := by
          have plain : ¬ projectionHasTag (next :: tail) := by
            rintro ⟨other, rest, same, major⟩
            cases same
            exact chained major
          have accepted : ¬ projectionTagRejected tag (next :: tail) := fun h => h.elim reserved plain
          simp only [projectionTagResult, if_neg accepted]
        isimp only [resultEq] at HΦ
        iapply HΦ
        iframe Hc Hs
        ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_checked
