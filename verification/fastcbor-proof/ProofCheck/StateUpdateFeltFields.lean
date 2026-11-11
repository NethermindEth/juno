import ProofCheck.StateUpdateErrors

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

/-- Successful BlockHash decoding publishes exactly the child value and
    preserves all sibling fields and their allocations. -/
theorem wp_stateUpdate_blockHash_valid (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (valid : FeltValid d item value) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"BlockHash" dst)))
    {{ RET #interface.nil; StateUpdateOwned dst {old with blockHash := value} ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_blockHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFelt_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %value [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro; exact Hnext

/-- Every BlockHash failure preserves the entire previous record and
    returns an initialized owned diagnostic at the exact input offset. -/
theorem wp_stateUpdate_blockHash_rejected (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"BlockHash" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗ c ↦ v ∗
       v.buf' ↦*{dq} xs ∗ StateUpdateFieldFailureOwned err (v.pos' + W64 offset) (.felt reason) ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  icases stateUpdate_blockHash_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  wp_apply wp_readFelt_failure_bounded c _ previous v xs wire dq d offset reason (feltFailureKind reason kinds)
    failure hd depth hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hkind Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.blockHash [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateUpdateFieldFailureOwned
    iexists kinds
    iframe Hglobals Herr
  ipureintro; exact Hnext

/-- Successful NewRoot decoding publishes exactly the child value and
    preserves all sibling fields and their allocations. -/
theorem wp_stateUpdate_newRoot_valid (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (valid : FeltValid d item value) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"NewRoot" dst)))
    {{ RET #interface.nil; StateUpdateOwned dst {old with newRoot := value} ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_newRoot_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFelt_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %value [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro; exact Hnext

/-- Every NewRoot failure preserves the entire previous record and
    returns an initialized owned diagnostic at the exact input offset. -/
theorem wp_stateUpdate_newRoot_rejected (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"NewRoot" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗ c ↦ v ∗
       v.buf' ↦*{dq} xs ∗ StateUpdateFieldFailureOwned err (v.pos' + W64 offset) (.felt reason) ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  icases stateUpdate_newRoot_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  wp_apply wp_readFelt_failure_bounded c _ previous v xs wire dq d offset reason (feltFailureKind reason kinds)
    failure hd depth hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hkind Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.newRoot [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateUpdateFieldFailureOwned
    iexists kinds
    iframe Hglobals Herr
  ipureintro; exact Hnext

/-- Successful OldRoot decoding publishes exactly the child value and
    preserves all sibling fields and their allocations. -/
theorem wp_stateUpdate_oldRoot_valid (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (valid : FeltValid d item value) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"OldRoot" dst)))
    {{ RET #interface.nil; StateUpdateOwned dst {old with oldRoot := value} ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_oldRoot_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFelt_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %value [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro; exact Hnext

/-- Every OldRoot failure preserves the entire previous record and
    returns an initialized owned diagnostic at the exact input offset. -/
theorem wp_stateUpdate_oldRoot_rejected (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (failure : FeltFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate go!"OldRoot" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗ c ↦ v ∗
       v.buf' ↦*{dq} xs ∗ StateUpdateFieldFailureOwned err (v.pos' + W64 offset) (.felt reason) ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  icases stateUpdate_oldRoot_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  wp_apply wp_readFelt_failure_bounded c _ previous v xs wire dq d offset reason (feltFailureKind reason kinds)
    failure hd depth hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hkind Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.oldRoot [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateUpdateFieldFailureOwned
    iexists kinds
    iframe Hglobals Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_blockHash_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_blockHash_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_newRoot_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_newRoot_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_oldRoot_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_oldRoot_rejected
