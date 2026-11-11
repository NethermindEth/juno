import ProofCheck.FieldTextComplete
import ProofCheck.SierraEntryPointTable

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

/-- Full-schema fresh/unknown key selection for arbitrary supported encodings.
    The actual validator contract remains an explicit unresolved premise. -/
theorem wp_nextSierraEntryPointKey_valid (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs rem : List w8) (wire : List w8) (choice : Option SierraEntryPointField) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ sierraEntryPointFieldKeys.length)
    (valid : SierraEntryPointKeyValid wire choice)
    (fresh : SierraEntryPointKeyFresh s.seen' choice)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ f.names' ↦*{qn} sierraEntryPointFieldNames ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(sierraEntryPointKeyCode choice) #interface.nil); ∃ next : w64,
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := next, seen' := sierraEntryPointKeySeen s.seen' choice} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ f.names' ↦*{qn} sierraEntryPointFieldNames ∗ v.buf' ↦*{dq} xs ∗
       ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ sierraEntryPointFieldKeys.length⌝ ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  obtain ⟨decoded, valid, meaning⟩ := valid
  cases valid with
  | @integer head payload argument enc major bound =>
    have noneEq : choice = none := meaning
    subst choice
    wp_apply wp_nextField_integer c st fields v s f sierraEntryPointFieldKeys sierraEntryPointFieldNames xs payload rem head argument qf qo dq
      hfields hleft hstart sierraEntryPointFieldEncodings_nonempty sierraEntryPointFieldHeads_ordered sierraEntryPointCachedShortTable sierraEntryPointInlineFieldTable
      enc major bound hp hbytes $$ [Hc Hst Hfields Horder Hs] with ⟨Hc, Hst, Hfields, Horder, Hs, %Hnext⟩
    · iframe Hc Hst Hfields Horder Hs
    isimp only [sierraEntryPointKeyCode] at HΦ
    iapply HΦ
    iexists s.next'
    isimp only [sierraEntryPointKeyCode, sierraEntryPointKeySeen, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
    iframe Hc Hst Hfields Horder Hnames Hs
    ipureintro
    refine ⟨hstart, ?_⟩
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext
  | @text head payload key argument enc major count utf8 =>
    have argumentEq : argument = W64 key.length := by word
    subst argument
    wp_apply wp_nextField_text_valid validator c st fields v s f sierraEntryPointFieldKeys sierraEntryPointFieldNames xs payload key rem head kind qf qo qn dq
      hfields hleft hstart sierraEntryPointFieldEncodings_nonempty sierraEntryPointFieldHeads_ordered sierraEntryPointFieldEncodings_apart sierraEntryPointCachedShortTable
      sierraEntryPointInlineFieldTable (by decide) enc major hp hbytes utf8
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨%next, Hc, Hfields, Horder, Hnames, Hs, %Hnext, %Hsearch, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    have hnext := fieldSearchAdvance_bounds sierraEntryPointFieldKeys ((head :: payload) ++ key ++ rem) s.next' next hstart (by change (2 : Nat) < 2^63; decide) Hsearch
    have hlen : (((head :: payload) ++ key).length : Int) = 1 + payload.length + key.length := by simp; omega
    cases choice with
    | none =>
      ihave Hout := fieldReadOutcome_unknown st {s with left' := s.left' - W64 1, next' := next} sierraEntryPointFieldNames key code err kind
        (v.pos' + W64 (1 + payload.length + key.length)) (sierraEntryPointKeyMeaning_unknown key meaning) $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [sierraEntryPointKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [sierraEntryPointKeyCode, sierraEntryPointKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext
    | some field =>
      ihave Hout := fieldReadOutcome_known st {s with left' := s.left' - W64 1, next' := next} sierraEntryPointFieldNames key
        (sierraEntryPointFieldName field) (sierraEntryPointKeyIndex field) code err kind (v.pos' + W64 (1 + payload.length + key.length))
        sierraEntryPointFoldedNames_distinct (by cases field <;> decide) (sierraEntryPointKey_lookup field) meaning $$ Hout
      ihave Hout := knownFieldOutcome_fresh st {s with left' := s.left' - W64 1, next' := next}
        (sierraEntryPointKeyIndex field) (sierraEntryPointFieldName field) code err kind (v.pos' + W64 (1 + payload.length + key.length)) fresh $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [sierraEntryPointKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [sierraEntryPointKeyCode, sierraEntryPointKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext

/-- Full-schema duplicate rejection for arbitrary supported encodings.
    The actual validator contract remains an explicit unresolved premise. -/
theorem wp_nextSierraEntryPointKey_duplicate_valid (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs rem : List w8) (wire : List w8) (field : SierraEntryPointField) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ sierraEntryPointFieldKeys.length)
    (valid : SierraEntryPointKeyValid wire (some field))
    (seen : s.seen' &&& (W64 1 <<< sierraEntryPointKeyIndex field) ≠ W64 0)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ f.names' ↦*{qn} sierraEntryPointFieldNames ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ (next : w64) (args : GoSlice),
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := next} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ f.names' ↦*{qn} sierraEntryPointFieldNames ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 wire.length) (go!"field %s named twice") args ∗
       args ↦* [interface.mkOk go.string #(sierraEntryPointFieldName field)] ∗
       ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ sierraEntryPointFieldKeys.length⌝ ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  obtain ⟨decoded, valid, meaning⟩ := valid
  cases valid with
  | integer enc major bound => cases meaning
  | @text head payload key argument enc major count utf8 =>
    have argumentEq : argument = W64 key.length := by word
    subst argument
    wp_apply wp_nextField_text_valid validator c st fields v s f sierraEntryPointFieldKeys sierraEntryPointFieldNames xs payload key rem head kind qf qo qn dq
      hfields hleft hstart sierraEntryPointFieldEncodings_nonempty sierraEntryPointFieldHeads_ordered sierraEntryPointFieldEncodings_apart sierraEntryPointCachedShortTable
      sierraEntryPointInlineFieldTable (by decide) enc major hp hbytes utf8
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨%next, Hc, Hfields, Horder, Hnames, Hs, %Hnext, %Hsearch, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    have hnext := fieldSearchAdvance_bounds sierraEntryPointFieldKeys ((head :: payload) ++ key ++ rem) s.next' next hstart (by change (2 : Nat) < 2^63; decide) Hsearch
    have hlen : (((head :: payload) ++ key).length : Int) = 1 + payload.length + key.length := by simp; omega
    ihave Hout := fieldReadOutcome_known st {s with left' := s.left' - W64 1, next' := next} sierraEntryPointFieldNames key
      (sierraEntryPointFieldName field) (sierraEntryPointKeyIndex field) code err kind (v.pos' + W64 (1 + payload.length + key.length))
      sierraEntryPointFoldedNames_distinct (by cases field <;> decide) (sierraEntryPointKey_lookup field) meaning $$ Hout
    ihave Hout := knownFieldOutcome_duplicate st {s with left' := s.left' - W64 1, next' := next}
      (sierraEntryPointKeyIndex field) (sierraEntryPointFieldName field) code err kind (v.pos' + W64 (1 + payload.length + key.length)) seen $$ Hout
    icases Hout with ⟨%errLoc, %args, %Heq, Hst, Herr, Hargs⟩
    rcases Heq with ⟨rfl, rfl⟩
    iapply HΦ
    iexists next, args
    isimp only [hlen]
    iframe Hc Hst Hfields Horder Hnames Hs Herr Hargs
    ipureintro
    simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext

/-- Every independently rejected SierraEntryPoint key preserves the cursor and search
    state, decrements the pair count, and owns the exact parser diagnostic. -/
theorem wp_nextSierraEntryPointKey_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs wire : List w8) (qf qo dq : DFrac) (reason : KeyFailureReason) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ sierraEntryPointFieldKeys.length)
    (failure : KeyRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} sierraEntryPointFieldKeys ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  exact wp_nextField_rejected_utf8 validator c st fields v s f sierraEntryPointFieldKeys sierraEntryPointFieldNames
    xs wire qf qo dq reason kind hfields hleft hstart sierraEntryPointFieldEncodings_nonempty
    sierraEntryPointFieldHeads_ordered sierraEntryPointCachedShortTable sierraEntryPointInlineFieldTable sierraEntryPointFieldNames_ascii failure hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextSierraEntryPointKey_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextSierraEntryPointKey_duplicate_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextSierraEntryPointKey_rejected
