import ProofCheck.ProjectionTagSuccess
import ProofCheck.ReceiptStatusBoundary
import ProofCheck.ReceiptStatusOwnership
import ProofCheck.ProjectionInitialize
import ProofCheck.ProjectionTagRead
import ProofCheck.StructOpenSound
import ProofCheck.WantedBoundary

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- After an accepted optional tag, opening errors return before field dispatch. The real initialized
    projection table supplies the opening wrapper's pointer; no metadata
    initializer or opening-helper correctness is assumed. -/
theorem wp_readReceiptStatus_open_failure_after_tag (d : Nat) (c dst : Loc)
    (old : ReceiptStatusValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : FeltFailureReason) (category : GoInterface)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (initial : cursor.Cursor) (input : List w8) (tagConsumed : Nat) (kinds : FeltErrorKinds)
    (tag : ProjectionTagSpec input tagConsumed (wire) .success)
    (initialPos : 0 ≤ sint.Z initial.pos' ∧ sint.Z initial.pos' ≤ sint.Z initial.buf'.len)
    (initialBytes : subslice (sint.nat initial.pos') (sint.nat initial.buf'.len) xs = input)
    (advance : {initial with pos' := initial.pos' + W64 tagConsumed} = v) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       FeltErrorGlobals kinds ∗ feltFailureAddr (GF := GF) reason ↦□ category ∗
       ReceiptStatusOwned dst old ∗ c ↦ initial ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason category }} := by
  iintro %Φ ⟨#Hpkg, #Hglobals, #Hcategory, Howned, Hc, Hs⟩ HΦ
  ihave #Htable := projection_initialized_table (GF := GF) .receiptExecutionStatus $$ Hpkg
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  all_goals
    isimp only [projectionGlobal] at Hfields
    wp_func_call
    wp_call
    wp_auto
    have sameBuf : initial.buf' = v.buf' := by rw [← advance]
    wp_apply wp_projectionTag_success c initial xs input (wire) dq tagConsumed kinds tag initialPos initialBytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Htag⟩
    · isimp only [sameBuf]
      iframe Hglobals Hc Hs
    isimp only [advance] at Hc
    isimp only [sameBuf] at Hs
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_openStruct_failure c fields v xs wire dq d reason category failure hd hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hcategory Hc Hs
    wp_for
    wp_end


/-- After an accepted optional tag, null and undefined consume one byte, preserve the prior projection value,
    and finish through the actual closed-state NextWanted branch. -/
theorem wp_readReceiptStatus_nullish_after_tag (c dst : Loc)
    (old : ReceiptStatusValue) (v : cursor.Cursor) (xs rest : List w8) (head : w8) (dq : DFrac)
    (nullish : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (initial : cursor.Cursor) (input : List w8) (tagConsumed : Nat) (kinds : FeltErrorKinds)
    (tag : ProjectionTagSpec input tagConsumed (head :: rest) .success)
    (initialPos : 0 ≤ sint.Z initial.pos' ∧ sint.Z initial.pos' ≤ sint.Z initial.buf'.len)
    (initialBytes : subslice (sint.nat initial.pos') (sint.nat initial.buf'.len) xs = input)
    (advance : {initial with pos' := initial.pos' + W64 tagConsumed} = v) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       FeltErrorGlobals kinds ∗ ReceiptStatusOwned dst old ∗ c ↦ initial ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; ReceiptStatusOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hglobals, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  ihave #Htable := projection_initialized_table (GF := GF) .receiptExecutionStatus $$ Hpkg
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  all_goals
    isimp only [projectionGlobal] at Hfields
    wp_func_call
    wp_call
    wp_auto
    have sameBuf : initial.buf' = v.buf' := by rw [← advance]
    wp_apply wp_projectionTag_success c initial xs input (head :: rest) dq tagConsumed kinds tag initialPos initialBytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Htag⟩
    · isimp only [sameBuf]
      iframe Hglobals Hc Hs
    isimp only [advance] at Hc
    isimp only [sameBuf] at Hs
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_openStruct_nullish c fields v xs head dq hp.1 hi nullish $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_for
    wp_apply wp_nextWanted_done c _ fields (W64 0) (W64 0) $$ [s] with Hst
    · iframe
    wp_for_post
    wp_end

/-- Reserved tags and chained tags return the actual non-nil unsupported
    sentinel directly from the generated receipt projection reader. No field is
    published and only the first tag head is consumed. -/
theorem wp_readReceiptStatus_tag_unsupported (c dst : Loc)
    (old : ReceiptStatusValue) (v : cursor.Cursor) (xs wire rest : List w8) (dq : DFrac)
    (consumed : Nat) (kinds : FeltErrorKinds) (tag : ProjectionTagSpec wire consumed rest .unsupported)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       FeltErrorGlobals kinds ∗ ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ RET #kinds.unsupportedKind;
       ReceiptStatusOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗ v.buf' ↦*{dq} xs ∗
       ⌜kinds.unsupportedKind ≠ interface.nil ∧
         sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  all_goals
    wp_start as ⟨#Hglobals, Howned, Hc, Hs⟩
    ihave %HnonNil := projection_unsupported_nonNil kinds $$ [Hglobals]
    · iframe Hglobals
      solve_pkg_init
    wp_auto
    wp_apply wp_projectionTag_spec c v xs wire rest dq consumed .unsupported kinds tag hp hbytes
      $$ [Hglobals Hc Hs] as %result ⟨Hc, Hs, Herror, %Hnext⟩
    · iframe Hglobals Hc Hs
    isimp only [ProjectionTagError] at Herror
    icases Herror with %Hresult
    subst result
    cases category : kinds.unsupportedKind with
    | nil => exact False.elim (HnonNil category)
    | ok info =>
      wp_auto
      wp_end
      iframe Howned Hc Hs
      ipureintro
      exact ⟨by simp, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_open_failure_after_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_nullish_after_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_tag_unsupported
