import ProofCheck.SkipHeadFailure

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

/-- Every recursive malformed-head derivation is realized by the actual worker.
    Container and tag child obligations are discharged by induction, with no
    assumed reader contracts or shortcut exclusions left in the statement. -/
theorem wp_skip_headFailure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (bad : w8) (reason : HeadError) (kind : GoInterface) (dq : DFrac)
    (depth : w64) (d offset : Nat) (failure : SkipHeadFailure bad reason d wire offset)
    (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 offset} ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 offset) bad reason kind }} := by
  induction failure generalizing v depth with
  | fault tail fault =>
    have hz : v.pos' + W64 0 = v.pos' := by word
    simpa only [Int.natCast_zero, hz] using wp_skip_fault (GF := GF) c v depth xs tail bad dq reason kind fault hp hbytes
  | @container d offset head payload tail count perCount items enc hm bound more fit valid child ih =>
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    let pos := v.pos' + W64 (1 + payload.length) + W64 items.flatten.length
    let out := v.pos' + W64 (1 + payload.length + items.flatten.length + offset)
    have Hnext := input_consumption xs ((head :: payload) ++ items.flatten) tail v.pos' v.buf'.len hp Hlen.1
      (by simpa only [List.append_assoc] using hbytes)
    have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
      simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      dsimp only [pos]; constructor <;> word
    have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail := by
      have hsum : pos = v.pos' + W64 ((head :: payload) ++ items.flatten).length := by
        dsimp only [pos]; simp only [List.length_append, List.length_cons]; word
      rw [hsum]; exact Hnext.2.2
    have hfinal : pos + W64 offset = out := by dsimp only [pos, out]; word
    have childSpec := ih {v with pos' := pos} (depth + W64 1) (by word) (by omega) hpos hremaining
    wp_apply wp_skip_container_prefix_reject c v xs payload tail head count perCount depth out items dq d
      iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err out bad reason kind)
      enc hm hd bound more fit valid
      (skipHeadFailure_not_wide bad reason d _ _ (SkipHeadFailure.container items enc hm bound more fit valid child))
      (skipHeadFailure_not_wide bad reason (d + 1) offset tail child)
      (by simpa only [hfinal] using childSpec) hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @tag d offset head next payload tail argument enc hm number bound child ih =>
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    let pos := v.pos' + W64 (1 + payload.length)
    let out := v.pos' + W64 (1 + payload.length + offset)
    have Hnext := input_consumption xs (head :: payload) (next :: tail) v.pos' v.buf'.len hp Hlen.1 hbytes
    have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      dsimp only [pos]; constructor <;> word
    have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = next :: tail := by
      simpa only [pos, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hnext.2.2
    let childDepth := depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0)
    have hdc : sint.Z childDepth = (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) := by
      dsimp only [childDepth]
      by_cases ht : next &&& W8 224 = W8 192 <;> simp only [ht, ite_true, ite_false] <;> word
    have hbd : (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) ≤ 32 := by
      by_cases ht : next &&& W8 224 = W8 192
      · have := bound ht; simp only [ht, ite_true]; omega
      · simp only [ht, ite_false, Nat.add_zero]; exact hdepth
    have hfinal : pos + W64 offset = out := by dsimp only [pos, out]; word
    have childSpec := ih {v with pos' := pos} childDepth hdc hbd hpos hremaining
    wp_apply wp_skip_tag_reject c v xs payload tail [] head next argument depth out dq
      iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err out bad reason kind)
      enc hm number ⟨by omega, by omega⟩ (by intro ht; have := bound ht; omega)
      (by simpa only [hfinal] using childSpec) hp (by simpa only [List.append_nil] using hbytes)
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- Public Skip restores the entire original cursor on recursive malformed-head
    failures and owns the exact inner diagnostic at a nonwrapping input offset. -/
theorem wp_Skip_headFailure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (bad : w8) (reason : HeadError) (kind : GoInterface) (dq : DFrac)
    (d offset : Nat) (failure : SkipHeadFailure bad reason d wire offset)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err (v.pos' + W64 offset) bad reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) < sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hoff := skipHeadFailure_offset bad reason d offset wire failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hpos : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) < sint.Z v.buf'.len := by constructor <;> word
  wp_auto
  wp_apply wp_skip_headFailure c v xs wire bad reason kind dq v.depth' d offset failure hd hdepth hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_headFailure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_headFailure
