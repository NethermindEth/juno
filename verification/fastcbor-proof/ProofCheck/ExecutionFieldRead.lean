import ProofCheck.ExecutionOwnership

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

/-- Exactly the generated switch's five calls, including explicit supplied slots. -/
def executionFieldCall (field : ExecutionField) (c dst availability gas : Loc) : Expr :=
  match field with
  | .builtins => (App (App (Val (@! cborReadBuiltinInstanceCounter)) (Val #c))
      (Val #(structFieldRef ExecutionResources go!"BuiltinInstanceCounter" dst)))
  | .holes => (App (App (Val (@! cborReadUint64)) (Val #c))
      (Val #(structFieldRef ExecutionResources go!"MemoryHoles" dst)))
  | .steps => (App (App (Val (@! cborReadUint64)) (Val #c))
      (Val #(structFieldRef ExecutionResources go!"Steps" dst)))
  | .availability => (App (App (App (Val (@! cborReadDataAvailabilitySlot)) (Val #c))
      (Val #(structFieldRef ExecutionResources go!"DataAvailability" dst))) (Val #availability))
  | .gas => (App (App (App (Val (@! cborReadGasConsumedSlot)) (Val #c))
      (Val #(structFieldRef ExecutionResources go!"TotalGasConsumed" dst))) (Val #gas))

def ExecutionFieldFailureOwned (err : Loc) (pos : w64) : ExecutionFieldFailure → IProp GF
  | .builtins reason => BuiltinsFailureOwned err pos reason
  | .availability reason => AvailabilityFailureOwned err pos reason
  | .gas reason => GasUsedFailureOwned err pos reason
  | .scalar reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))

theorem wp_execution_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst availability gas : Loc) (old final : ExecutionValue) (field : ExecutionField)
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : ExecutionFieldValid d field old item rest final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ExecutionOwned dst availability gas old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (executionFieldCall (GF := GF) field c dst availability gas)
    {{ RET #interface.nil; ExecutionOwned dst availability gas final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hreceipts, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [ExecutionOwned] at Howned
  icases Howned with ⟨Hdst, Ha, Hg⟩
  cases valid with
  | @builtins old item rest value valid =>
    simp only [executionFieldCall]
    icases execution_builtins_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readBuiltins_record_valid validator c _ old.builtins value v xs item rest dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · isimp only [BuiltinsOwned]; iframe Hfield Hc Hs
    isimp only [BuiltinsOwned] at Hfield
    ihave Hdst := Hclose $$ %(builtinsConcrete value) Hfield
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete]
    iframe Hdst Ha Hg Hc Hs
    ipureintro; exact Hnext
  | @holes old item rest value valid =>
    simp only [executionFieldCall]
    icases execution_holes_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef ExecutionResources go!"MemoryHoles" dst)
      old.holes v xs item rest value dq valid hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete]
    iframe Hdst Ha Hg Hc Hs
    ipureintro; exact Hnext
  | @steps old item rest value valid =>
    simp only [executionFieldCall]
    icases execution_steps_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef ExecutionResources go!"Steps" dst)
      old.steps v xs item rest value dq valid hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hdst := Hclose $$ %value Hfield
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete]
    iframe Hdst Ha Hg Hc Hs
    ipureintro; exact Hnext
  | @availability old item rest value valid =>
    simp only [executionFieldCall]
    icases execution_availability_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readAvailabilitySlot_valid validator c _ (if old.availabilityPresent then availability else null) availability old.availabilityStorage
      v xs item rest value dq d valid hd hp hbytes $$ [Hfield Ha Hc Hs]
      as %out ⟨Hfield, Ha, %Hout, Hc, Hs, %Hnext⟩
    · iframe Hfield Ha Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    have heq : {executionConcrete availability gas old with DataAvailability' := out} =
        executionConcrete availability gas (executionSetAvailability old value) := by
      rw [Hout]
      rfl
    isimp only [heq] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned]
    isplitl [Hdst Ha Hg]
    · iframe Hdst
      isimp only [executionSetAvailability]
      iframe Ha Hg
    iframe Hc Hs
    ipureintro; exact Hnext
  | @gas old item rest value valid =>
    simp only [executionFieldCall]
    icases execution_gas_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readGasUsedSlot_valid validator c _ (if old.gasPresent then gas else null) gas old.gasStorage
      v xs item rest value dq d valid hd hp hbytes $$ [Hfield Hg Hc Hs]
      as %out ⟨Hfield, Hg, %Hout, Hc, Hs, %Hnext⟩
    · iframe Hfield Hg Hc Hs
    ihave Hdst := Hclose $$ %out Hfield
    have heq : {executionConcrete availability gas old with TotalGasConsumed' := out} =
        executionConcrete availability gas (executionSetGas old value) := by
      rw [Hout]
      rfl
    isimp only [heq] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned]
    isplitl [Hdst Ha Hg]
    · iframe Hdst
      isimp only [executionSetGas]
      iframe Ha Hg
    iframe Hc Hs
    ipureintro; exact Hnext

/-- Failure exposes the exact partial record and slot contents, not merely the
    old destination pointer. All retained cursor levels and diagnostics are owned. -/
theorem wp_execution_field_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst availability gas : Loc) (old retained : ExecutionValue) (field : ExecutionField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat)
    (reason : ExecutionFieldFailure)
    (failure : ExecutionFieldRejected d field old wire retained consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ExecutionOwned dst availability gas old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (executionFieldCall (GF := GF) field c dst availability gas)
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ExecutionOwned dst availability gas retained ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ ExecutionFieldFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hreceipts, #Hschema, Howned, Hc, Hs⟩ HΦ
  isimp only [ExecutionOwned] at Howned
  icases Howned with ⟨Hdst, Ha, Hg⟩
  cases failure with
  | @builtins old wire value consumed offset entered reason failure =>
    simp only [executionFieldCall]
    icases execution_builtins_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readBuiltins_record_rejected validator c _ old.builtins value v xs wire dq d consumed offset entered reason
      failure hd hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · isimp only [BuiltinsOwned]; iframe Hfield Hc Hs
    isimp only [BuiltinsOwned] at Hfield
    ihave Hdst := Hclose $$ %(builtinsConcrete value) Hfield
    have he : (if entered then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if entered then 1 else 0 : Nat) := by
      cases entered <;> simp <;> word
    isimp only [he] at Hc
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete, ExecutionFieldFailureOwned]
    iframe Hdst Ha Hg Hc Hs Herr
    ipureintro; exact Hnext
  | @availability old wire value consumed offset entered reason failure =>
    simp only [executionFieldCall]
    icases execution_availability_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readAvailabilitySlot_rejected validator c _ (if old.availabilityPresent then availability else null) availability old.availabilityStorage
      v xs wire value dq d consumed offset entered reason failure hd hp hbytes $$ [Hfield Ha Hc Hs]
      as %err ⟨Hfield, Ha, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Ha Hc Hs
    ihave Hdst := Hclose $$ %(if old.availabilityPresent then availability else null) Hfield
    have he : (if entered then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if entered then 1 else 0 : Nat) := by
      cases entered <;> simp <;> word
    isimp only [he] at Hc
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete, ExecutionFieldFailureOwned]
    iframe Hdst Ha Hg Hc Hs Herr
    ipureintro; exact Hnext
  | @gas old wire value consumed offset entered reason failure =>
    simp only [executionFieldCall]
    icases execution_gas_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    wp_apply wp_readGasUsedSlot_rejected validator c _ (if old.gasPresent then gas else null) gas old.gasStorage
      v xs wire value dq d consumed offset entered reason failure hd hp hbytes $$ [Hfield Hg Hc Hs]
      as %err ⟨Hfield, Hg, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hg Hc Hs
    ihave Hdst := Hclose $$ %(if old.gasPresent then gas else null) Hfield
    have he : (if entered then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if entered then 1 else 0 : Nat) := by
      cases entered <;> simp <;> word
    isimp only [he] at Hc
    isimp only [executionConcrete] at Hdst
    iapply HΦ
    isimp only [ExecutionOwned, executionConcrete, ExecutionFieldFailureOwned]
    iframe Hdst Ha Hg Hc Hs Herr
    ipureintro; exact Hnext
  | @holes old wire reason failure =>
    simp only [executionFieldCall]
    icases execution_holes_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
    icases Hglobals with ⟨%kinds, #Hglobals⟩
    have hread := wp_readScalar_complete (GF := GF) false c (structFieldRef ExecutionResources go!"MemoryHoles" dst)
      old.holes v xs wire dq kinds hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] as %result ⟨Hs, Hout⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [ScalarOutcome] at Hout
    icases Hout with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, Hfield, Hc, %Hresult⟩
      rw [Hresult.2.1] at failure
      exact False.elim (scalarValid_not_failure _ _ _ _ _ Hresult.2.2.1 failure)
    · icases Hfailure with ⟨%actual, %err, Hfield, Hc, Herr, %Hresult⟩
      have eq := scalarFailure_reason_unique _ _ _ failure Hresult.2
      subst actual
      rcases Hresult with ⟨rfl, _⟩
      ihave Hdst := Hclose $$ %old.holes Hfield
      isimp only [executionConcrete] at Hdst
      iapply HΦ
      isimp only [ExecutionOwned, executionConcrete, ExecutionFieldFailureOwned, Int.natCast_zero,
        Int.add_zero, show v.pos' + W64 0 = v.pos' from by word, show v.depth' + W64 0 = v.depth' from by word]
      iframe Hdst Ha Hg Hc Hs
      isplitl [Herr]
      · iexists kinds; iframe Hglobals Herr
      ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @steps old wire reason failure =>
    simp only [executionFieldCall]
    icases execution_steps_acc dst (executionConcrete availability gas old) $$ Hdst with ⟨Hfield, Hclose⟩
    isimp only [executionConcrete] at Hfield
    ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
    icases Hglobals with ⟨%kinds, #Hglobals⟩
    have hread := wp_readScalar_complete (GF := GF) false c (structFieldRef ExecutionResources go!"Steps" dst)
      old.steps v xs wire dq kinds hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] as %result ⟨Hs, Hout⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [ScalarOutcome] at Hout
    icases Hout with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, Hfield, Hc, %Hresult⟩
      rw [Hresult.2.1] at failure
      exact False.elim (scalarValid_not_failure _ _ _ _ _ Hresult.2.2.1 failure)
    · icases Hfailure with ⟨%actual, %err, Hfield, Hc, Herr, %Hresult⟩
      have eq := scalarFailure_reason_unique _ _ _ failure Hresult.2
      subst actual
      rcases Hresult with ⟨rfl, _⟩
      ihave Hdst := Hclose $$ %old.steps Hfield
      isimp only [executionConcrete] at Hdst
      iapply HΦ
      isimp only [ExecutionOwned, executionConcrete, ExecutionFieldFailureOwned, Int.natCast_zero,
        Int.add_zero, show v.pos' + W64 0 = v.pos' from by word, show v.depth' + W64 0 = v.depth' from by word]
      iframe Hdst Ha Hg Hc Hs
      isplitl [Herr]
      · iexists kinds; iframe Hglobals Herr
      ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_execution_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_execution_field_rejected
