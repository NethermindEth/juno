import ProofCheck.HeaderProjectionOwnership

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

/-- Borrow the actual TransactionCount cell while retaining the embedded skeleton. -/
theorem headerProjectionCount_acc (dst : Loc) (h : headerTransactionCountProjection) :
    dst ↦ h ⊢ (structFieldRef headerTransactionCountProjection go!"TransactionCount" dst ↦ h.TransactionCount' ∗
      (∀ value : w64, structFieldRef headerTransactionCountProjection go!"TransactionCount" dst ↦ value -∗
        dst ↦ {h with TransactionCount' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hfield, -⟩
  iframe Hfield
  iintro %value Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

/-- Borrow the actual Timestamp cell while retaining the embedded skeleton. -/
theorem headerProjectionTimestamp_acc (dst : Loc) (h : headerTimestampProjection) :
    dst ↦ h ⊢ (structFieldRef headerTimestampProjection go!"Timestamp" dst ↦ h.Timestamp' ∗
      (∀ value : Loc, structFieldRef headerTimestampProjection go!"Timestamp" dst ↦ value -∗
        dst ↦ {h with Timestamp' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hskeleton, Hfield, -⟩
  iframe Hfield
  iintro %value Hvalue
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe

theorem headerProjectionChildren_set_count (h : headerTransactionCountProjection) (old : HeaderProjectionValue) (value : w64) :
    HeaderProjectionChildren .count h old ⊢
      (HeaderProjectionChildren .count {h with TransactionCount' := value} {old with count := value} : IProp GF) := by
  unfold HeaderProjectionChildren
  iintro ⟨%Hcanonical, _⟩
  ipureintro
  exact ⟨Hcanonical, rfl⟩

theorem headerProjectionChildren_set_timestamp (h : headerTimestampProjection) (old : HeaderProjectionValue)
    (p : Loc) (value : Option w64) :
    HeaderProjectionChildren .timestamp h old ∗ Uint64PointerResult p value ⊢
      (HeaderProjectionChildren .timestamp {h with Timestamp' := p} {old with timestamp := value} : IProp GF) := by
  unfold HeaderProjectionChildren
  iintro ⟨⟨%Hcanonical, _⟩, Hvalue⟩
  iframe Hvalue
  ipureintro
  exact Hcanonical

/-- Actual count decoding preserves an existing scalar on null/undefined and
    replaces it for unsigned values, with exact input consumption and suffix. -/
theorem wp_headerProjectionCount_valid (c dst : Loc) (h : headerTransactionCountProjection)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire rem : List w8) (value : w64) (dq : DFrac)
    (valid : ScalarValid old.count wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ HeaderProjectionChildren .count h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #(structFieldRef headerTransactionCountProjection go!"TransactionCount" dst)))
    {{ RET #interface.nil; dst ↦ {h with TransactionCount' := value} ∗
       HeaderProjectionChildren .count {h with TransactionCount' := value} {old with count := value} ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hchildren, Hc, Hs⟩
  isimp only [HeaderProjectionChildren] at Hchildren
  icases Hchildren with ⟨%Hcanonical, %Hcount⟩
  have fieldValid : ScalarValid h.TransactionCount' wire value := by rw [Hcount]; exact valid
  icases headerProjectionCount_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef headerTransactionCountProjection go!"TransactionCount" dst)
    h.TransactionCount' v xs wire rem value dq fieldValid hp hbytes
  simp only [Bool.false_eq_true, ite_false] at hread
  wp_apply hread $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %value Hfield
  ihave Hchildren : HeaderProjectionChildren .count {h with TransactionCount' := value} {old with count := value} $$ []
  · unfold HeaderProjectionChildren
    ipureintro
    exact ⟨Hcanonical, rfl⟩
  wp_end

/-- Actual nullable timestamp decoding clears on null/undefined and owns a
    fresh word on unsigned input, including allocated zero. -/
theorem wp_headerProjectionTimestamp_valid (c dst : Loc) (h : headerTimestampProjection)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire rem : List w8) (value : Option w64) (dq : DFrac)
    (valid : Uint64PointerValid wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ h ∗ HeaderProjectionChildren .timestamp h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64Pointer)) (Val #c)) (Val #(structFieldRef headerTimestampProjection go!"Timestamp" dst)))
    {{ RET #interface.nil; ∃ p : Loc, dst ↦ {h with Timestamp' := p} ∗
       HeaderProjectionChildren .timestamp {h with Timestamp' := p} {old with timestamp := value} ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hchildren, Hc, Hs⟩
  icases headerProjectionTimestamp_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readUint64Pointer_valid c (structFieldRef headerTimestampProjection go!"Timestamp" dst)
    h.Timestamp' v xs wire rem value dq valid hp hbytes $$ [Hfield Hc Hs] with ⟨%p, Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %p Hfield
  ihave Hchildren := headerProjectionChildren_set_timestamp h old p value $$ [Hchildren Hvalue]
  · iframe Hchildren Hvalue
  iapply HΦ
  iexists p
  iframe Hh Hchildren Hc Hs
  ipureintro
  exact Hnext

/-- A rejected count value preserves the concrete destination, cursor and
    previous child ownership, returning the exact owned diagnostic. -/
theorem wp_headerProjectionCount_failure (c dst : Loc) (h : headerTransactionCountProjection)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : ExpectedFailureReason) (kind : GoInterface) (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗
       HeaderProjectionChildren .count h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #(structFieldRef headerTransactionCountProjection go!"TransactionCount" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderProjectionChildren .count h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hchildren, Hc, Hs⟩
  icases headerProjectionCount_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  have hread := wp_readScalar_failure (GF := GF) false c (structFieldRef headerTransactionCountProjection go!"TransactionCount" dst)
    h.TransactionCount' v xs wire dq reason kind failure hp hbytes
  simp only [Bool.false_eq_true, ite_false] at hread
  wp_apply hread $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %h.TransactionCount' Hfield
  wp_end

/-- A rejected timestamp value preserves the concrete destination, cursor and
    previous child ownership, returning the exact owned diagnostic. -/
theorem wp_headerProjectionTimestamp_failure (c dst : Loc) (h : headerTimestampProjection)
    (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : ExpectedFailureReason) (kind : GoInterface) (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗
       HeaderProjectionChildren .timestamp h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64Pointer)) (Val #c)) (Val #(structFieldRef headerTimestampProjection go!"Timestamp" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderProjectionChildren .timestamp h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hchildren, Hc, Hs⟩
  icases headerProjectionTimestamp_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readUint64Pointer_failure c (structFieldRef headerTimestampProjection go!"Timestamp" dst)
    h.Timestamp' v xs wire dq reason kind failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %h.Timestamp' Hfield
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionCount_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionTimestamp_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_set_count
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_set_timestamp
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionCount_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionTimestamp_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionCount_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionTimestamp_failure
