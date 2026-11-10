import ProofCheck.SkipElementsReject

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

/-- A sequence of valid ignored items followed by a non-wide head cannot match
    a longer sequence of wide unsigned heads at nine-byte intervals. -/
theorem skipPrefix_not_wide_heads (items : List (List w8)) (head : w8) (rem : List w8)
    (d n : Nat) (valid : ∀ item ∈ items, SkipValid d item) (hne : head ≠ W8 27)
    (hcount : items.length < n)
    (hheads : ∀ i : Nat, i < n → (items.flatten ++ (head :: rem))[9 * i]? = some (W8 27)) : False := by
  induction items generalizing n with
  | nil =>
    have hh : head = W8 27 := by simpa using hheads 0 (by simpa using hcount)
    exact hne hh
  | cons item items ih =>
    have hn : 0 < n := by simp only [List.length_cons] at hcount; omega
    have hv := valid item (by simp)
    have hnonempty := skipValid_nonempty d item hv
    have hraw : (item :: items).flatten ++ (head :: rem) = item ++ (items.flatten ++ (head :: rem)) := by
      simp only [List.flatten_cons, List.append_assoc]
    have hh : item[0]? = some (W8 27) := by
      have hh := hheads 0 hn
      simpa only [hraw, Nat.mul_zero, List.getElem?_append, hnonempty, ite_true] using hh
    have hwidth := skipValid_wide_uint d item hv hh
    apply ih (n - 1) (fun x hx => valid x (by simp only [List.mem_cons]; exact Or.inr hx))
      (by simp only [List.length_cons] at hcount; omega)
    intro i hi
    have hg := hheads (i + 1) (by omega)
    rw [hraw] at hg
    simpa only [List.getElem?_append, hwidth, show ¬ 9 * (i + 1) < 9 by omega,
      ite_false, show 9 * (i + 1) - 9 = 9 * i by omega] using hg

/-- A malformed child after valid skipped contents rules out the enclosing
    four-wide-limb shortcut for every array/map header width. -/
theorem skip_container_fault_not_wide (head bad : w8) (payload rem : List w8)
    (count perCount : w64) (items : List (List w8)) (d : Nat) (reason : HeadError)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hmore : (items.length : Int) < uint.Z count * uint.Z perCount)
    (valid : ∀ item ∈ items, SkipValid d item) (fault : HeadFault bad reason) :
    ¬ SkipWideShape ((head :: payload) ++ (items.flatten ++ (bad :: rem))) := by
  intro shape
  have heq : head = W8 132 := by simpa using shape.1
  subst head
  obtain ⟨rfl, hc⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
  rcases hm with ⟨_, rfl⟩ | ⟨hm, _⟩
  · have hinfo := headFault_info bad reason fault
    have hne : bad ≠ W8 27 := by intro h; subst bad; contradiction
    have hcount : items.length < 4 := by rw [hc] at hmore; change (items.length : Int) < 4 * 1 at hmore; omega
    apply skipPrefix_not_wide_heads items bad rem d 4 valid hne hcount
    intro i hi
    have hc : i = 0 ∨ i = 1 ∨ i = 2 ∨ i = 3 := by omega
    rcases hc with rfl | rfl | rfl | rfl
    · simpa using shape.2.1
    · simpa using shape.2.2.1
    · simpa using shape.2.2.2.1
    · simpa using shape.2.2.2.2
  · contradiction

/-- Actual array/map dispatch and iteration compose valid children with a proved
    later failure. Count checks bound multiplication before the loop begins. -/
theorem wp_skip_container_prefix_reject (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (count perCount depth out : w64)
    (items : List (List w8)) (dq : DFrac) (d : Nat) (R : IProp GF) (E : Loc → IProp GF)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : sint.Z depth = d) (hdepth : d < 32)
    (hmore : (items.length : Int) < uint.Z count * uint.Z perCount)
    (hfit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ rem).length)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item)
    (shape : ¬ SkipWideShape ((head :: payload) ++ (items.flatten ++ rem)))
    (badShape : ¬ SkipWideShape rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length) + W64 items.flatten.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(depth + W64 1)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }})
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (items.flatten ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start_folded as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) (items.flatten ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_append, List.length_cons] at hlen
  let pos := v.pos' + W64 (1 + payload.length)
  have hstart : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = items.flatten ++ rem := by
    simpa only [pos, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hnext.2.2
  have hremain : uint.Z (v.buf'.len - pos) = (items.flatten ++ rem).length := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; simp only [List.length_append]; word
  have htotal : uint.Z count * uint.Z perCount ≤ sint.Z v.buf'.len := by
    have hrbound : uint.Z (v.buf'.len - pos) ≤ sint.Z v.buf'.len := by word
    omega
  have hlimit : uint.Z count ≤ uint.Z ((v.buf'.len - pos) / perCount) := by
    rcases hm with ⟨_, rfl⟩ | ⟨_, rfl⟩ <;> word
  have hper : perCount = W64 1 ∨ perCount = W64 2 := hm.elim (fun h => Or.inl h.2) (fun h => Or.inr h.2)
  wp_apply wp_skip_container_reject c v xs payload (items.flatten ++ rem) head count perCount depth out dq R E enc hm
    (wp_skipElements_prefix_reject c {v with pos' := pos} xs rem items dq count
      ((v.buf'.len - pos) / perCount) perCount depth out d R E hd hdepth hper hmore htotal hlimit valid badShape
      hbad hstart hremaining) hp hbytes (fun _ => shape)
    $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- Closed malformed-child contract for arrays/maps after arbitrary valid nested
    contents. Every child proof is supplied by the proved acceptance or head reader. -/
theorem wp_skip_container_fault (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count perCount depth : w64)
    (items : List (List w8)) (dq : DFrac) (d : Nat) (reason : HeadError) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : sint.Z depth = d) (hdepth : d < 32)
    (hmore : (items.length : Int) < uint.Z count * uint.Z perCount)
    (hfit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ (bad :: rem)).length)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item) (fault : HeadFault bad reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (items.flatten ++ (bad :: rem))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length) + W64 items.flatten.length} ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 (1 + payload.length) + W64 items.flatten.length) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length) + W64 items.flatten.length
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatten) (bad :: rem) v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have hpos : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = bad :: rem := by
    have hsum : pos = v.pos' + W64 ((head :: payload) ++ items.flatten).length := by
      dsimp only [pos]; simp only [List.length_append, List.length_cons]; word
    rw [hsum]; exact Hnext.2.2
  have hinfo := headFault_info bad reason fault
  have badShape : ¬ SkipWideShape (bad :: rem) := by
    intro shape
    have heq : bad = W8 132 := by simpa using shape.1
    subst bad; contradiction
  wp_apply wp_skip_container_prefix_reject c v xs payload (bad :: rem) head count perCount depth pos items dq d
    iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err pos bad reason kind)
    enc hm hd hdepth hmore hfit valid
    (skip_container_fault_not_wide head bad payload rem count perCount items (d + 1) reason enc hm hmore valid fault)
    badShape (wp_skip_fault c {v with pos' := pos} (depth + W64 1) xs rem bad dq reason kind fault hpos hremaining)
    hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip rolls the whole container back on a malformed later child,
    preserving its exact inner diagnostic and all input bytes. -/
theorem wp_Skip_container_fault (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count perCount : w64)
    (items : List (List w8)) (dq : DFrac) (d : Nat) (reason : HeadError) (kind : GoInterface)
    (enc : HeadArgument head payload count)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hmore : (items.length : Int) < uint.Z count * uint.Z perCount)
    (hfit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ (bad :: rem)).length)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item) (fault : HeadFault bad reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (items.flatten ++ (bad :: rem))) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err
         (v.pos' + W64 (1 + payload.length) + W64 items.flatten.length) bad reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_container_fault c v xs payload rem head bad count perCount v.depth' items dq d reason kind
    enc hm hd hdepth hmore hfit valid fault hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipPrefix_not_wide_heads
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_container_fault_not_wide
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_container_prefix_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_container_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_container_fault
