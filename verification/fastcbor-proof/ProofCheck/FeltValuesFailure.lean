import ProofCheck.FeltValuesValid

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Every independent slice failure returns nil, preserves the complete cursor
    and owns the exact diagnostic, including count arguments and inner offsets. -/
theorem wp_limbSlice_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d offset : Nat) (reason : FeltValuesFailureReason) (kinds : FeltErrorKinds)
    (failure : FeltValuesSpec d wire (.failure offset reason)) (hd : sint.Z v.depth' = d)
    (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltValuesFailureError err (v.pos' + W64 offset) reason kinds ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hglobals, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hoff := feltValues_failure_bound d offset wire reason failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have Hbound : sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by constructor <;> word
  have hz (x : w64) : x + W64 0 = x := by word
  cases failure with
  | outer hn hu failure =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 128) _) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_limbSlice_outer_failure c v xs wire dq _ _ hn hu failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [FeltValuesFailureError, Int.natCast_zero, hz]
    iframe Hc Hs Herr
    ipureintro; simpa only [Int.natCast_zero, hz] using Hbound
  | @count head payload tail argument count enc major size short =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 128) .tag) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_limbSlice_count c v xs payload tail head argument dq kinds.unsupportedKind enc major hp.1
      (by omega) (by omega) Hhead.2.1 Hhead.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, %args, Herr, Hargs⟩
    · iframe Hkind Hc Hs
    have harg : argument = W64 count := by word
    have hleft : v.buf'.len - (v.pos' + W64 (1 + payload.length)) = W64 tail.length := by word
    isimp only [harg, hleft] at Hargs
    iapply HΦ
    isimp only [FeltValuesFailureError, Int.natCast_zero, hz]
    iframe Hc Hs
    isplitl [Herr Hargs]
    · iexists args; iframe Herr Hargs
    ipureintro; simpa only [Int.natCast_zero, hz] using Hbound
  | @depth head payload tail argument count enc major size fit deep =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds .depth $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_limbSlice_depth c v xs payload tail head argument dq kinds.depthKind enc major hp.1
      (by omega) (by omega) (by omega) Hhead.2.1 Hhead.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [FeltValuesFailureError, Int.natCast_zero, hz]
    iframe Hc Hs Herr
    ipureintro; simpa only [Int.natCast_zero, hz] using Hbound
  | @elementDepth head payload tail argument count enc major size fit depth nonempty deep =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds .depth $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_limbSlice_elementDepth c v xs payload tail head argument dq kinds.depthKind enc major hp.1
      (by omega) (by omega) (by omega) (by omega) Hhead.2.1 Hhead.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [FeltValuesFailureError, Int.natCast_zero, hz]
    iframe Hc Hs Herr
    ipureintro; simpa only [Int.natCast_zero, hz] using Hbound
  | @element head payload tail argument done consumed offset reason count enc major size fit depth children failure =>
    have nonempty := limbItems_rejected_bounds (d + 1) count consumed offset tail done reason failure
    have harg : argument = W64 count := by word
    subst argument
    ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_limbSlice_nonempty_rejected c v xs payload tail done head dq d count consumed offset reason
      (feltFailureKind reason kinds) enc major (by omega) fit failure hd (by omega) hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [FeltValuesFailureError, Int.natCast_add, Int.natCast_one]
    iframe Hc Hs Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using Hbound

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_failure
