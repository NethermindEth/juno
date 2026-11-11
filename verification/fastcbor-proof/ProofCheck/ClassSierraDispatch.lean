import ProofCheck.SierraClassRecordComplete
import ProofCheck.ClassTagProfile
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

/-- The actual tag switch allocates a fresh SierraClass and publishes
    the correctly typed interface only after the complete record succeeds. -/
theorem wp_class_sierra_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORClassTags) (v : cursor.Cursor)
    (xs payload item rest : List w8) (head : w8) (value : SierraClassValue) (dq : DFrac) (d : Nat)
    (distinct : ClassTagsDistinct tags)
    (enc : HeadArgument head payload tags.SierraClass') (major : head &&& W8 224 = W8 192)
    (valid : SierraClassRecordValid d sierraClassZero item rest value) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (out : Loc), RET #interface.nil;
       dst ↦ interface.mkOk (go.PointerType SierraClass.ty) #out ∗ SierraClassOwned out value ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + item.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + item.length)) = sint.Z v.pos' + 1 + payload.length + item.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + item.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + item.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hn : ((head :: payload) ++ item ++ rest).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have hu : ((head :: payload) ++ item ++ rest).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have HDeprecatedClass : tags.SierraClass' ≠ tags.DeprecatedCairoClass' := distinct .sierra .deprecated (by decide)
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ item ++ rest) dq hn hu hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_tag_wire c v xs payload (item ++ rest) head tags.SierraClass' dq enc major hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Htag⟩
  · iframe Hc Hs
  simp only [HDeprecatedClass, _root_.decide_false]
  wp_auto
  ihave Howned := sierraClassOwned_zero «$r0_ptr» $$ «$r0»
  wp_apply wp_readSierraClass_record_valid validator c «$r0_ptr» sierraClassZero value
    {v with pos' := v.pos' + W64 (1 + payload.length)} xs item rest dq d valid hd
    ⟨by rw [Htag.1]; omega, Htag.2.1⟩ Htag.2.2 $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
  · iframe Howned Hc Hs
  have hpos : v.pos' + W64 (1 + payload.length) + W64 item.length = v.pos' + W64 (1 + payload.length + item.length) := by word
  isimp only [hpos] at Hc
  iapply HΦ $$ %«$r0_ptr»
  iframe Hdst Howned Hc Hs
  ipureintro
  rw [← hpos]
  exact ⟨by rw [Hnext.1, Htag.1], Hnext.2⟩

/-- A tagged SierraClass failure keeps its fresh partial record private and
    preserves the caller's old interface, with exact consumption and error. -/
theorem wp_class_sierra_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORClassTags) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (retained : SierraClassValue) (dq : DFrac)
    (d consumed offset : Nat) (extraDepth : Nat) (reason : SierraClassFailureReason)
    (distinct : ClassTagsDistinct tags)
    (enc : HeadArgument head payload tags.SierraClass') (major : head &&& W8 224 = W8 192)
    (failure : SierraClassRecordRejected d sierraClassZero wire retained consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ (∃ temporary : Loc, SierraClassOwned temporary retained) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SierraClassFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) reason ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hn : ((head :: payload) ++ wire).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have hu : ((head :: payload) ++ wire).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have HDeprecatedClass : tags.SierraClass' ≠ tags.DeprecatedCairoClass' := distinct .sierra .deprecated (by decide)
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ wire) dq hn hu hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_tag_wire c v xs payload (wire) head tags.SierraClass' dq enc major hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Htag⟩
  · iframe Hc Hs
  simp only [HDeprecatedClass, _root_.decide_false]
  wp_auto
  ihave Howned := sierraClassOwned_zero «$r0_ptr» $$ «$r0»
  wp_apply wp_readSierraClass_record_rejected validator c «$r0_ptr» sierraClassZero retained
    {v with pos' := v.pos' + W64 (1 + payload.length)} xs wire dq d consumed offset extraDepth reason failure hd
    ⟨by rw [Htag.1]; omega, Htag.2.1⟩ Htag.2.2 $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hstep⟩
  · iframe Howned Hc Hs
  have hpos : v.pos' + W64 (1 + payload.length) + W64 consumed = v.pos' + W64 (1 + payload.length + consumed) := by word
  have hoff : v.pos' + W64 (1 + payload.length) + W64 offset = v.pos' + W64 (1 + payload.length + offset) := by word
  isimp only [hpos] at Hc
  isimp only [hoff] at Herr
  iapply HΦ
  iframe Hdst Hc Hs Herr
  isplitl [Howned]
  · iexists «$r0_ptr»; iexact Howned
  ipureintro
  rw [← hpos, ← hoff]
  exact ⟨by rw [Hstep.1, Htag.1], by rw [Hstep.2.1, Htag.1], Hstep.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_sierra_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_sierra_rejected
