import ProofCheck.SkipFailureContext

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

/-- Actual enclosing readers propagate a proved leaf failure. The recursive
    calls and both wide shortcuts are discharged by induction and wire lemmas. -/
theorem wp_skip_context_reject (c : Loc) (v : cursor.Cursor) (xs front tail : List w8)
    (dq : DFrac) (depth out : w64) (d leaf : Nat) (R : IProp GF) (E : Loc → IProp GF)
    (context : SkipFailureContext tail d front leaf)
    (short : SkipNonWideUint tail) (rejected : SkipWideRejected leaf tail)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ tail)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 leaf)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  induction context generalizing v depth with
  | nil d =>
    have hz : v.pos' + W64 ([] : List w8).length = v.pos' := by simp only [List.length_nil]; word
    have hdep : W64 d = depth := by word
    simpa only [hz, hdep] using hbad
  | @container d leaf head payload front count perCount items enc hm bound more fit valid child ih =>
    wp_start_folded as ⟨HR, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    let pos := v.pos' + W64 (1 + payload.length) + W64 items.flatten.length
    have Hnext := input_consumption xs ((head :: payload) ++ items.flatten) (front ++ tail) v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_assoc] using hbytes)
    have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
      simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      dsimp only [pos]; constructor <;> word
    have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = front ++ tail := by
      have hsum : pos = v.pos' + W64 ((head :: payload) ++ items.flatten).length := by
        dsimp only [pos]; simp only [List.length_append, List.length_cons]; word
      rw [hsum]; exact Hnext.2.2
    have hfinal : pos + W64 front.length = v.pos' + W64 ((head :: payload) ++ (items.flatten ++ front)).length := by
      dsimp only [pos]; simp only [List.length_append, List.length_cons]; word
    have childSpec := ih {v with pos' := pos} (depth + W64 1) rejected (by word) (by omega) hpos hremaining
      (by simpa only [hfinal] using hbad)
    have shape := skipFailureContext_wideRejected tail ((head :: payload) ++ (items.flatten ++ front)) d leaf
      (SkipFailureContext.container items enc hm bound more fit valid child) short rejected
    wp_apply wp_skip_container_prefix_reject c v xs payload (front ++ tail) head count perCount depth out items dq d R E
      enc hm hd bound more fit valid
      (by simpa only [List.append_assoc] using shape bound)
      (skipFailureContext_wideRejected tail front (d + 1) leaf child short rejected)
      childSpec hp (by simpa only [List.append_assoc] using hbytes)
      $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end
  | @tag d leaf head next payload front rest argument enc hm number nonempty bound child ih =>
    wp_start_folded as ⟨HR, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    let pos := v.pos' + W64 (1 + payload.length)
    have Hnext := input_consumption xs (head :: payload) (front ++ tail) v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_assoc] using hbytes)
    have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      dsimp only [pos]; constructor <;> word
    have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = front ++ tail := by
      simpa only [pos, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hnext.2.2
    let childDepth := depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0)
    have hdc : sint.Z childDepth = (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) := by
      dsimp only [childDepth]
      by_cases ht : next &&& W8 224 = W8 192 <;> simp only [ht, ite_true, ite_false] <;> word
    have hbd : (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) ≤ 32 := by
      by_cases ht : next &&& W8 224 = W8 192
      · have := bound ht; simp only [ht, ite_true]; omega
      · simp only [ht, ite_false, Nat.add_zero]; exact hdepth
    have hfinal : pos + W64 front.length = v.pos' + W64 ((head :: payload) ++ front).length := by
      dsimp only [pos]; simp only [List.length_append, List.length_cons]; word
    have childSpec := ih {v with pos' := pos} childDepth rejected hdc hbd hpos hremaining
      (by simpa only [hfinal] using hbad)
    wp_apply wp_skip_tag_reject c v xs payload rest [] head next argument depth out dq R E
      enc hm number ⟨by omega, by omega⟩ (by intro ht; have := bound ht; omega)
      childSpec hp (by simpa only [List.append_nil, List.append_assoc, nonempty] using hbytes)
      $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end

/-- Public Skip preserves a leaf diagnostic while rolling back all enclosing
    contexts. This is an adapter requiring an explicit proved leaf contract. -/
theorem wp_Skip_context_reject (c : Loc) (v : cursor.Cursor) (xs front tail : List w8)
    (dq : DFrac) (out : w64) (d leaf : Nat) (R : IProp GF) (E : Loc → IProp GF)
    (context : SkipFailureContext tail d front leaf)
    (short : SkipNonWideUint tail) (rejected : SkipWideRejected leaf tail)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ tail)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 leaf)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_context_reject c v xs front tail dq v.depth' out d leaf R E context short rejected
    hd hdepth hp hbytes hbad $$ [Hc Hs HR] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_context_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_reject
