import ProofCheck.FeltMapFromBoundary
import ProofCheck.FeltMapOwnership
import ProofCheck.FeltMapSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- An allocated empty map owns no felt children. -/
theorem feltMapOwned_empty (out : Loc) :
    (out ↦$ (∅ : GMap FeltMapKey Loc)) ⊢ (FeltMapOwned out ∅ : IProp GF) := by
  iintro Hmap
  unfold FeltMapOwned
  iexists (∅ : GMap FeltMapKey Loc)
  iframe Hmap
  iapply feltMapChildren_empty

/-- An empty definite map is allocated and published with exact consumption.
    All encoded widths are covered. No map insertion or key equality is used. -/
theorem wp_readFeltMapFrom_empty (c dst : Loc) (old spare : Loc) (reserve : GoSlice) (extra : w64) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (hm : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       spare ↦ reserve ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadFeltMapFrom)) (Val #c)) (Val #dst)) (Val #spare)) (Val #extra))
    {{ (out : Loc), RET #interface.nil; spare ↦ reserve ∗ dst ↦ out ∗ FeltMapOwned out ∅ ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) =
           sint.Z v.pos' + 1 + payload.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hspare, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hi := input_suffix_head xs (payload ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 246 &&& W8 224 ≠ W8 160) hc
  have hu : head ≠ W8 247 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 247 &&& W8 224 ≠ W8 160) hc
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ rem := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload rem head (W64 0) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply (wp_map_make2 (K := FeltMapKey) (V := Loc) (W64 0)) with %out Hmap
  ihave Howned := feltMapOwned_empty out $$ Hmap
  wp_for
  wp_apply wp_leave c {v with pos' := v.pos' + W64 (1 + payload.length)} $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ $$ %out
  iframe Hspare Hdst Howned Hc Hs
  ipureintro
  simpa only [List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

/-- An empty definite map is allocated and published with exact consumption.
    All encoded widths are covered. No map insertion or key equality is used. -/
theorem wp_readFeltMap_empty (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (hm : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ FeltMapOwned out ∅ ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) =
           sint.Z v.pos' + 1 + payload.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_readFeltMapFrom_empty c dst old spare_ptr slice.nil (W64 0) v xs payload rem head dq d
    enc hm hd hdepth hp hbytes $$ [spare Hdst Hc Hs] as %out ⟨Hspare, Hdst, Hnew, Hc, Hs, %Hnext⟩
  · iframe spare Hdst Hc Hs
  iapply HΦ $$ %out
  iframe Hdst Hnew Hc Hs
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMapFrom_empty

#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapOwned_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMap_empty
