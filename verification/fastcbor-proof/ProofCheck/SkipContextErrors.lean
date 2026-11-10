import ProofCheck.SkipContextSound

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

/-- Incomplete argument heads do not begin a complete wide uint and cannot
    trigger the felt shortcut, even when the incomplete head itself is 0x1b. -/
theorem skip_truncated_shortcuts (head : w8) (rem : List w8) (size : w64) (d : Nat)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size) :
    SkipNonWideUint (head :: rem) ∧ SkipWideRejected d (head :: rem) := by
  constructor
  · intro hh
    have heq : head = W8 27 := by simpa using hh
    subst head
    change W64 8 = size at hsize
    rw [← hsize] at htrunc
    change (rem.length : Int) < 8 at htrunc
    simp only [List.length_cons]; omega
  · intro hd hlen shape
    have heq : head = W8 132 := by simpa using shape.1
    subst head; contradiction

/-- An incomplete head reached through arbitrary supported arrays/maps/tags
    retains its starting offset while public Skip restores all cursor fields. -/
theorem wp_Skip_context_truncated (c : Loc) (v : cursor.Cursor)
    (xs front rem : List w8) (head : w8) (size : w64) (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (context : SkipFailureContext (head :: rem) d front leaf)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : rem.length < sint.Z size)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (head :: rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 front.length) (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (head :: rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have guards := skip_truncated_shortcuts head rem size leaf hinfo hsize htrunc
  wp_apply wp_Skip_context_reject c v xs front (head :: rem) dq (v.pos' + W64 front.length) d leaf
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends inside the head") slice.nil))
    context guards.1 guards.2 hd hdepth hp hbytes
    (wp_skip_head_truncated c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs rem head size dq kind
      hinfo hsize htrunc hpos Hnext.2.2) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Atomic rejections cannot be mistaken for a wide unsigned value or a felt. -/
theorem skip_atomic_shortcuts (head : w8) (payload rem : List w8) (argument : w64) (reason : SkipAtomError) (d : Nat)
    (reject : SkipReject head argument rem reason) :
    SkipNonWideUint ((head :: payload) ++ rem) ∧ SkipWideRejected d ((head :: payload) ++ rem) := by
  constructor
  · intro hh
    have heq : head = W8 27 := by simpa using hh
    subst head
    cases reject with
    | simple hm _ _ => contradiction
    | bytes hm _ => rcases hm with h | h <;> contradiction
    | tag hm _ => contradiction
  · intro hd hlen shape
    have heq : head = W8 132 := by simpa using shape.1
    exact skipReject_not_felt head argument rem reason reject heq

/-- Atomic failures reached through arbitrary supported containers and tags
    preserve their post-head diagnostic offset and restore the public cursor. -/
theorem wp_Skip_context_atomic (c : Loc) (v : cursor.Cursor)
    (xs front payload rem : List w8) (head : w8) (argument : w64) (dq : DFrac) (d leaf : Nat)
    (reason : SkipAtomError) (kind : GoInterface)
    (context : SkipFailureContext ((head :: payload) ++ rem) d front leaf)
    (enc : HeadArgument head payload argument) (reject : SkipReject head argument rem reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ ((head :: payload) ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 front.length + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front ((head :: payload) ++ rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have guards := skip_atomic_shortcuts head payload rem argument reason leaf reject
  wp_apply wp_Skip_context_reject c v xs front ((head :: payload) ++ rem) dq
    (v.pos' + W64 front.length + W64 (1 + payload.length)) d leaf iprop(skipAtomErrorAddr (GF := GF) reason ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil))
    context guards.1 guards.2 hd hdepth hp hbytes
    (wp_skip_reject c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs payload rem head argument dq reason kind
      enc reject hpos Hnext.2.2) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- EOF after valid siblings is reported at the original end of input and is
    propagated through every enclosing context without leaking cursor progress. -/
theorem wp_Skip_context_eof (c : Loc) (v : cursor.Cursor) (xs front : List w8)
    (dq : DFrac) (d leaf : Nat) (kind : GoInterface)
    (context : SkipFailureContext [] d front leaf)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind
         (v.pos' + W64 front.length) (go!"input ends before the item") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega)] at hlen
  have heof : v.pos' + W64 front.length = v.buf'.len := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen
    word
  have hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 leaf)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := v.pos' + W64 front.length} ∗ v.buf' ↦*{dq} xs ∗
         err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends before the item") slice.nil }} := by
    wp_start_folded as ⟨#Hkind, Hc, Hs⟩
    wp_apply wp_skip_eof c {v with pos' := v.pos' + W64 front.length} (W64 leaf) kind heof Hwf
      $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    wp_end
  wp_apply wp_Skip_context_reject c v xs front [] dq (v.pos' + W64 front.length) d leaf
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind (v.pos' + W64 front.length) (go!"input ends before the item") slice.nil))
    context (by intro h; contradiction) (by intro hd hl; simp only [List.length_nil] at hl; omega)
    hd hdepth hp (by simpa only [List.append_nil] using hbytes) hbad $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_truncated_shortcuts
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_truncated
#print axioms Perennial.github_com.NethermindEth.juno.core.skip_atomic_shortcuts
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_atomic
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_eof
