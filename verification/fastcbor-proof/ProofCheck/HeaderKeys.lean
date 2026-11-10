import ProofCheck.FieldChoice

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

/-- Independent identities for all fifteen Header schema fields. Wire names
    honor the two legacy cbor tags; identities are separate from Go field names. -/
inductive HeaderField where
  | hash
  | parentHash
  | number
  | globalStateRoot
  | sequencerAddress
  | transactionCount
  | eventCount
  | timestamp
  | protocolVersion
  | gasETH
  | signatures
  | gasSTRK
  | mode
  | dataGas
  | l2Gas
  deriving DecidableEq

def headerKeyIndex : HeaderField → w64
  | .hash => W64 0
  | .parentHash => W64 1
  | .number => W64 2
  | .globalStateRoot => W64 3
  | .sequencerAddress => W64 4
  | .transactionCount => W64 5
  | .eventCount => W64 6
  | .timestamp => W64 7
  | .protocolVersion => W64 8
  | .gasETH => W64 9
  | .signatures => W64 10
  | .gasSTRK => W64 11
  | .mode => W64 12
  | .dataGas => W64 13
  | .l2Gas => W64 14

def headerFieldWireName : HeaderField → GoString
  | .hash => go!"Hash"
  | .parentHash => go!"ParentHash"
  | .number => go!"Number"
  | .globalStateRoot => go!"GlobalStateRoot"
  | .sequencerAddress => go!"SequencerAddress"
  | .transactionCount => go!"TransactionCount"
  | .eventCount => go!"EventCount"
  | .timestamp => go!"Timestamp"
  | .protocolVersion => go!"ProtocolVersion"
  | .gasETH => go!"gasprice"
  | .signatures => go!"Signatures"
  | .gasSTRK => go!"gaspricestrk"
  | .mode => go!"L1DAMode"
  | .dataGas => go!"L1DataGasPrice"
  | .l2Gas => go!"L2GasPrice"

def headerKeyCode (choice : Option HeaderField) : w64 :=
  match choice with
  | none => W64 (-2)
  | some field => headerKeyIndex field

def headerKeySeen (seen : w64) (choice : Option HeaderField) : w64 :=
  match choice with
  | none => seen
  | some field => seen ||| (W64 1 <<< headerKeyIndex field)

def HeaderKeyFresh (seen : w64) (choice : Option HeaderField) : Prop :=
  match choice with
  | none => True
  | some field => seen &&& (W64 1 <<< headerKeyIndex field) = W64 0

def HeaderKeyTextMeaning (key : GoString) (choice : Option HeaderField) : Prop :=
  match choice with
  | none => ∀ name ∈ headerFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte
  | some field => key.map fieldFoldByte = (headerFieldWireName field).map fieldFoldByte

/-- The key families whose production contracts are currently closed. This is
    deliberately named Supported: wider and non-ASCII text are additional
    decoder-supported families still awaiting their own contracts. -/
inductive HeaderKeySupported : List w8 → Option HeaderField → Prop where
  | text {head : w8} {payload key : List w8} {choice : Option HeaderField}
      (enc : HeadArgument head payload (W64 key.length)) (major : head &&& W8 224 = W8 96)
      (width : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
      (ascii : ASCIIBytes key) (meaning : HeaderKeyTextMeaning key choice) :
      HeaderKeySupported ((head :: payload) ++ key) choice
  | integer {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : uint.Z argument < 2^63) : HeaderKeySupported (head :: payload) none

structure HeaderKeyWire where
  bytes : List w8
  choice : Option HeaderField
  valid : HeaderKeySupported bytes choice

theorem headerKey_lookup (field : HeaderField) :
    headerFieldNames[(sint.Z (headerKeyIndex field)).toNat]? = some (headerFieldWireName field) := by cases field <;> rfl

theorem headerKey_nonempty (key : HeaderKeyWire) : 0 < key.bytes.length := by
  rcases key with ⟨bytes, choice, valid⟩
  cases valid <;> simp only [List.length_append, List.length_cons] <;> omega

/-- Compose public field reading with schema meaning and a fresh seen bit. This
    closes the actual cache/search/marking calls for the currently proved key
    families and retains the search-index bound needed by the next iteration. -/
theorem wp_nextHeaderKey (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs rem : List w8) (key : HeaderKeyWire) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ headerFieldKeys.length)
    (fresh : HeaderKeyFresh s.seen' key.choice)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = key.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(headerKeyCode key.choice) #interface.nil); ∃ next : w64,
       c ↦ {v with pos' := v.pos' + W64 key.bytes.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := next, seen' := headerKeySeen s.seen' key.choice} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗ v.buf' ↦*{dq} xs ∗
       ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
       ⌜sint.Z (v.pos' + W64 key.bytes.length) = sint.Z v.pos' + key.bytes.length ∧
         sint.Z (v.pos' + W64 key.bytes.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 key.bytes.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  rcases key with ⟨bytes, choice, valid⟩
  dsimp only at fresh hbytes
  cases valid with
  | @integer head payload argument enc major bound =>
    wp_apply wp_nextField_integer c st fields v s f headerFieldKeys headerFieldNames xs payload rem head argument qf qo dq
      hfields hleft hstart headerFieldEncodings_nonempty headerFieldHeads_ordered headerCachedShortTable headerInlineFieldTable
      enc major bound hp hbytes $$ [Hc Hst Hfields Horder Hs] with ⟨Hc, Hst, Hfields, Horder, Hs, %Hnext⟩
    · iframe Hc Hst Hfields Horder Hs
    isimp only [HeaderKeyWire.choice, HeaderKeyWire.bytes, headerKeyCode] at HΦ
    iapply HΦ
    iexists s.next'
    isimp only [headerKeyCode, headerKeySeen, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
    iframe Hc Hst Hfields Horder Hnames Hs
    ipureintro
    refine ⟨hstart, ?_⟩
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext
  | @text head payload key choice enc major width ascii meaning =>
    wp_apply wp_nextField_ascii c st fields v s f headerFieldKeys headerFieldNames xs payload key rem head kind qf qo qn dq
      hfields hleft hstart headerFieldEncodings_nonempty headerFieldHeads_ordered headerFieldEncodings_apart headerCachedShortTable
      headerInlineFieldTable (by decide) enc major width ascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨%next, Hc, Hfields, Horder, Hnames, Hs, %Hnext, %Hsearch, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    have hnext := fieldSearchAdvance_bounds headerFieldKeys ((head :: payload) ++ key ++ rem) s.next' next hstart (by change (15 : Nat) < 2^63; decide) Hsearch
    have hlen : (((head :: payload) ++ key).length : Int) = 1 + payload.length + key.length := by simp; omega
    cases choice with
    | none =>
      ihave Hout := fieldReadOutcome_unknown st {s with left' := s.left' - W64 1, next' := next} headerFieldNames key code err kind
        (v.pos' + W64 (1 + payload.length + key.length)) meaning $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [HeaderKeyWire.choice, HeaderKeyWire.bytes, headerKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [headerKeyCode, headerKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext
    | some field =>
      ihave Hout := fieldReadOutcome_known st {s with left' := s.left' - W64 1, next' := next} headerFieldNames key
        (headerFieldWireName field) (headerKeyIndex field) code err kind (v.pos' + W64 (1 + payload.length + key.length))
        headerFoldedNames_distinct (by cases field <;> decide) (headerKey_lookup field) meaning $$ Hout
      ihave Hout := knownFieldOutcome_fresh st {s with left' := s.left' - W64 1, next' := next}
        (headerKeyIndex field) (headerFieldWireName field) code err kind (v.pos' + W64 (1 + payload.length + key.length)) fresh $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [HeaderKeyWire.choice, HeaderKeyWire.bytes, headerKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [headerKeyCode, headerKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.headerKey_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextHeaderKey
