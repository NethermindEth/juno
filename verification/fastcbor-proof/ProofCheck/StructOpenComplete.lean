import ProofCheck.StructOpenSound

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

/-- A record opening either consumes null/undefined without entering a map,
    enters a bounded definite map after consuming exactly its head, or rejects
    with its original cursor and a closed state. No field metadata is read. -/
def StructOpenOutcome (c fields : Loc) (v : cursor.Cursor) (xs wire : List w8) (d : Nat)
    (kinds : FeltErrorKinds) (state : cursor.Struct) (result : GoInterface) : IProp GF := iprop(
  (∃ (head : w8) (rest : List w8),
    c ↦ {v with pos' := v.pos' + W64 1} ∗
    ⌜wire = head :: rest ∧ (head = W8 246 ∨ head = W8 247) ∧
      state = {(zero_val cursor.Struct) with fields' := fields} ∧ result = interface.nil ∧
      sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (head : w8) (payload body : List w8) (count : w64),
    c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
    ⌜wire = (head :: payload) ++ body ∧ HeadArgument head payload count ∧ head &&& W8 224 = W8 160 ∧
      d < 32 ∧ 2 * uint.Z count ≤ body.length ∧
      state = cursor.Struct.mk fields count (W64 0) (W64 0) true ∧ result = interface.nil ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ∧
      sint.Z (v.depth' + W64 1) = d + 1⌝) ∨
  (∃ (reason : FeltFailureReason) (err : Loc), c ↦ v ∗
    FeltFailureError err v.pos' reason (feltFailureKind reason kinds) ∗
    ⌜StructOpenFailure d wire reason ∧ state = {(zero_val cursor.Struct) with fields' := fields} ∧
      result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝))

/-- Exhaustive actual OpenStruct behavior on arbitrary input at a supported
    depth. Null precedence, count-before-depth errors, and every head width are
    discharged by the independent classification and closed production contracts. -/
theorem wp_openStruct_complete (c fields : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (state : cursor.Struct) (result : GoInterface), RET (PairV #state #result);
       v.buf' ↦*{dq} xs ∗ StructOpenOutcome c fields v xs wire d kinds state result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hc, Hs⟩ HΦ
  rcases struct_open_classify d wire hdepth with ⟨head, rest, split, hn⟩ |
    ⟨head, payload, body, count, split, enc, major, depth, fit⟩ | ⟨reason, failure⟩
  · ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hb := hbytes.trans split
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hb
    have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hb
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    wp_apply wp_openStruct_nullish c fields v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    unfold StructOpenOutcome
    ileft
    iexists head, rest
    iframe Hc
    ipureintro
    exact ⟨split, hn, rfl, rfl, Hnext⟩
  · wp_apply wp_openStruct_map c fields v xs payload body head count dq d enc major hd depth fit hp (hbytes.trans split)
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    unfold StructOpenOutcome
    iright
    ileft
    iexists head, payload, body, count
    iframe Hc
    ipureintro
    exact ⟨split, enc, major, depth, fit, rfl, rfl, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_openStruct_failure c fields v xs wire dq d reason (feltFailureKind reason kinds) failure hd hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    iframe Hs
    unfold StructOpenOutcome
    iright
    iright
    iexists reason, err
    iframe Hc Herr
    ipureintro
    exact ⟨failure, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_complete
