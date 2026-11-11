import ProofCheck.HeaderProjectionOwnership
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

def headerProjectionReadName : HeaderProjectionKind → GoString
  | .hash => cborReadHeaderHashProjection
  | .root => cborReadHeaderGlobalStateRootProjection
  | .count => cborReadHeaderTransactionCountProjection
  | .timestamp => cborReadHeaderTimestampProjection
  | .hashAndRoot => cborReadHeaderHashAndStateRootProjection

/-- Malformed tag heads return directly from every actual generated projection
    record function, preserving its owned destination and the original cursor. -/
theorem wp_readHeaderProjection_tag_malformed (kind : HeaderProjectionKind) (c dst : Loc)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : ExpectedFailureReason) (category : GoInterface)
    (tagged : projectionHasTag wire) (failure : ExpectedFailure (W8 192) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       expectedFailureAddr (GF := GF) reason ↦□ category ∗ HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionReadName kind))) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 192) reason category }} := by
  cases kind <;> simp only [headerProjectionReadName]
  all_goals
    wp_start as ⟨#Hcategory, Howned, Hc, Hs⟩
    wp_auto
    wp_apply wp_projectionTag_malformed c v xs wire dq reason category tagged failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hcategory Hc Hs
    wp_end

/-- Untagged opening errors return before field dispatch. The real initialized
    projection table supplies the opening wrapper's pointer; no metadata
    initializer or opening-helper correctness is assumed. -/
theorem wp_readHeaderProjection_open_failure (d : Nat) (kind : HeaderProjectionKind) (c dst : Loc)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : FeltFailureReason) (category truncated : GoInterface)
    (plain : ¬ projectionHasTag wire) (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       globalAddr cursor.ErrTruncated ↦□ truncated ∗ feltFailureAddr (GF := GF) reason ↦□ category ∗
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionReadName kind))) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err v.pos' reason category }} := by
  iintro %Φ ⟨#Hpkg, #Htruncated, #Hcategory, Howned, Hc, Hs⟩ HΦ
  ihave #Htable := projection_initialized_table (GF := GF) (headerProjectionSchema kind) $$ Hpkg
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  cases kind <;> simp only [headerProjectionReadName]
  all_goals
    isimp only [headerProjectionSchema, projectionGlobal] at Hfields
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_projectionTag_untagged c v xs wire dq truncated plain hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Htruncated Hc Hs
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_openStruct_failure c fields v xs wire dq d reason category failure hd hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hcategory Hc Hs
    wp_for
    wp_end

/-- Null and undefined consume one byte, preserve the prior projection value,
    and finish through the actual closed-state NextWanted branch. -/
theorem wp_readHeaderProjection_nullish (kind : HeaderProjectionKind) (c dst : Loc)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs rest : List w8) (head : w8) (dq : DFrac)
    (truncated : GoInterface) (nullish : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       globalAddr cursor.ErrTruncated ↦□ truncated ∗ HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionReadName kind))) (Val #c)) (Val #dst))
    {{ RET #interface.nil; HeaderProjectionOwned kind dst old ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  have plain : ¬ projectionHasTag (head :: rest) := by
    rintro ⟨other, tail, same, major⟩
    simp only [List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    rcases nullish with rfl | rfl <;> contradiction
  iintro %Φ ⟨#Hpkg, #Htruncated, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  ihave #Htable := projection_initialized_table (GF := GF) (headerProjectionSchema kind) $$ Hpkg
  isimp only [ProjectionTable] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  cases kind <;> simp only [headerProjectionReadName]
  all_goals
    isimp only [headerProjectionSchema, projectionGlobal] at Hfields
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_projectionTag_untagged c v xs (head :: rest) dq truncated plain hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Htruncated Hc Hs
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderProjection_tag_malformed
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderProjection_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderProjection_nullish
