import ProofCheck.HeaderKeys
import ProofCheck.GasPriceDuplicate

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

/-- Duplicate known keys in the currently proved families consume the key,
    retain the existing seen bits, and return Done with the exact owned error.
    Values after the duplicate key are neither decoded nor skipped. -/
theorem wp_nextHeaderKey_duplicate (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs rem : List w8) (key : HeaderKeyWire) (field : HeaderField) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ headerFieldKeys.length)
    (choice : key.choice = some field)
    (seen : s.seen' &&& (W64 1 <<< headerKeyIndex field) ≠ W64 0)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = key.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ (next : w64) (args : GoSlice),
       c ↦ {v with pos' := v.pos' + W64 key.bytes.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := next} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 key.bytes.length) (go!"field %s named twice") args ∗
       args ↦* [interface.mkOk go.string #(headerFieldWireName field)] ∗
       ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
       ⌜sint.Z (v.pos' + W64 key.bytes.length) = sint.Z v.pos' + key.bytes.length ∧
         sint.Z (v.pos' + W64 key.bytes.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 key.bytes.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  rcases key with ⟨bytes, keyChoice, valid⟩
  dsimp only at choice hbytes
  cases valid with
  | integer enc major bound => cases choice
  | @text head payload key keyChoice enc major width ascii meaning =>
    subst keyChoice
    wp_apply wp_nextField_ascii c st fields v s f headerFieldKeys headerFieldNames xs payload key rem head kind qf qo qn dq
      hfields hleft hstart headerFieldEncodings_nonempty headerFieldHeads_ordered headerFieldEncodings_apart headerCachedShortTable
      headerInlineFieldTable (by decide) enc major width ascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨%next, Hc, Hfields, Horder, Hnames, Hs, %Hnext, %Hsearch, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    have hnext := fieldSearchAdvance_bounds headerFieldKeys ((head :: payload) ++ key ++ rem) s.next' next hstart (by change (15 : Nat) < 2^63; decide) Hsearch
    have hlen : (((head :: payload) ++ key).length : Int) = 1 + payload.length + key.length := by simp; omega
    ihave Hout := fieldReadOutcome_known st {s with left' := s.left' - W64 1, next' := next} headerFieldNames key
      (headerFieldWireName field) (headerKeyIndex field) code err kind (v.pos' + W64 (1 + payload.length + key.length))
      headerFoldedNames_distinct (by cases field <;> decide) (headerKey_lookup field) meaning $$ Hout
    ihave Hout := knownFieldOutcome_duplicate st {s with left' := s.left' - W64 1, next' := next}
      (headerKeyIndex field) (headerFieldWireName field) code err kind (v.pos' + W64 (1 + payload.length + key.length)) seen $$ Hout
    icases Hout with ⟨%errLoc, %args, %Heq, Hst, Herr, Hargs⟩
    rcases Heq with ⟨rfl, rfl⟩
    iapply HΦ
    iexists next, args
    isimp only [HeaderKeyWire.bytes, hlen]
    iframe Hc Hst Hfields Horder Hnames Hs Herr Hargs
    ipureintro
    simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextHeaderKey_duplicate
