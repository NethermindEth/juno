import ProofCheck.SkipContainerReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A container count exceeding the available bytes cannot have the wide-felt
    shape: its only possible array head would require four children but has fewer. -/
theorem skip_count_not_wide (head : w8) (payload rem : List w8) (count perCount : w64)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hshort : rem.length < uint.Z count * uint.Z perCount) :
    ¬ SkipWideShape ((head :: payload) ++ rem) := by
  intro shape
  have heq : head = W8 132 := by simpa using shape.1
  subst head
  obtain ⟨rfl, hcount⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
  rcases hm with ⟨_, rfl⟩ | ⟨hm, _⟩
  · rw [hcount] at hshort
    change (rem.length : Int) < 4 * 1 at hshort
    have hlen : rem.length < 4 := by omega
    have hnone : ((W8 132 :: []) ++ rem)[28]? = none := by
      apply List.getElem?_eq_none
      simp only [List.length_append, List.length_cons, List.length_nil]
      omega
    have hlast := shape.2.2.2.2
    rw [hnone] at hlast
    contradiction
  · contradiction

/-- At maximum depth, every complete array/map head is rejected before its
    count or children are considered. The diagnostic offset follows that head. -/
theorem wp_skip_container_depth (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count perCount : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 32)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  have bodySpec :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }}
        (App (App (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skipElements"))
          (Val #count)) (Val #((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perCount)))
          (Val #perCount)) (Val #(W64 32)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
         ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length))
           (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_skipElements_depth c {v with pos' := v.pos' + W64 (1 + payload.length)}
      count ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perCount) perCount kind $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    wp_end
  wp_apply wp_skip_container_reject c v xs payload rem head count perCount (W64 32)
    (v.pos' + W64 (1 + payload.length)) dq iprop(globalAddr cursor.ErrDepth ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length))
           (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)]))
    enc hm bodySpec hp hbytes (by intro h; change (32 : Int) < 32 at h; omega)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Every oversized array/map count is rejected without multiplication overflow,
    including inline-four arrays and unsigned counts beyond the signed range. -/
theorem wp_skip_container_count (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count perCount depth : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : 0 ≤ sint.Z depth ∧ sint.Z depth < 32)
    (hshort : rem.length < uint.Z count * uint.Z perCount)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_append, List.length_cons] at hlen
  have hremain : uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) = rem.length := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    word
  have hlimit : uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perCount) < uint.Z count := by
    rcases hm with ⟨_, rfl⟩ | ⟨_, rfl⟩ <;> word
  have bodySpec :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }}
        (App (App (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skipElements"))
          (Val #count)) (Val #((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perCount)))
          (Val #perCount)) (Val #depth))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
         err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_skipElements_count c {v with pos' := v.pos' + W64 (1 + payload.length)}
      count ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perCount) perCount depth kind hd hlimit
      $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    wp_end
  wp_apply wp_skip_container_reject c v xs payload rem head count perCount depth
    (v.pos' + W64 (1 + payload.length)) dq iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil))
    enc hm bodySpec hp hbytes (fun _ _ => skip_count_not_wide head payload rem count perCount enc hm hshort)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores the full cursor while retaining the container depth diagnostic. -/
theorem wp_Skip_container_depth (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count perCount : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : v.depth' = W64 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  simp only [hd]
  wp_apply wp_skip_container_depth c v xs payload rem head count perCount dq kind enc hm hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip restores the full cursor while retaining the container count diagnostic. -/
theorem wp_Skip_container_count (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count perCount : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : 0 ≤ sint.Z v.depth' ∧ sint.Z v.depth' < 32)
    (hshort : rem.length < uint.Z count * uint.Z perCount)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_container_count c v xs payload rem head count perCount v.depth' dq kind enc hm hd hshort hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_count_not_wide
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_container_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_container_count

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_container_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_container_count
