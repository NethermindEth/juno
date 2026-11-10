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

def gasKeyIndex (wei : Bool) : w64 := if wei then W64 0 else W64 1

def gasKeyCode (choice : Option Bool) : w64 :=
  match choice with
  | none => W64 (-2)
  | some wei => gasKeyIndex wei

def gasKeySeen (seen : w64) (choice : Option Bool) : w64 :=
  match choice with
  | none => seen
  | some wei => seen ||| (W64 1 <<< gasKeyIndex wei)

def GasKeyFresh (seen : w64) (choice : Option Bool) : Prop :=
  match choice with
  | none => True
  | some wei => seen &&& (W64 1 <<< gasKeyIndex wei) = W64 0

def GasKeyTextMeaning (key : GoString) (choice : Option Bool) : Prop :=
  match choice with
  | none => ∀ name ∈ gasFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte
  | some wei => key.map fieldFoldByte = (gasPriceFieldName wei).map fieldFoldByte

/-- The key families whose production contracts are currently closed. This is
    deliberately named Supported: wider and non-ASCII text are additional
    decoder-supported families still awaiting their own contracts. -/
inductive GasKeySupported : List w8 → Option Bool → Prop where
  | text {head : w8} {payload key : List w8} {choice : Option Bool}
      (enc : HeadArgument head payload (W64 key.length)) (major : head &&& W8 224 = W8 96)
      (width : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
      (ascii : ASCIIBytes key) (meaning : GasKeyTextMeaning key choice) :
      GasKeySupported ((head :: payload) ++ key) choice
  | integer {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : uint.Z argument < 2^63) : GasKeySupported (head :: payload) none

structure GasKeyWire where
  bytes : List w8
  choice : Option Bool
  valid : GasKeySupported bytes choice

theorem gasKey_lookup (wei : Bool) :
    gasFieldNames[(sint.Z (gasKeyIndex wei)).toNat]? = some (gasPriceFieldName wei) := by cases wei <;> rfl

theorem gasKey_nonempty (key : GasKeyWire) : 0 < key.bytes.length := by
  rcases key with ⟨bytes, choice, valid⟩
  cases valid <;> simp only [List.length_append, List.length_cons] <;> omega

/-- Compose public field reading with schema meaning and a fresh seen bit. This
    closes the actual cache/search/marking calls for the currently proved key
    families and retains the search-index bound needed by the next iteration. -/
theorem wp_nextGasKey (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (xs rem : List w8) (key : GasKeyWire) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ gasFieldKeys.length)
    (fresh : GasKeyFresh s.seen' key.choice)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = key.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(gasKeyCode key.choice) #interface.nil); ∃ next : w64,
       c ↦ {v with pos' := v.pos' + W64 key.bytes.length} ∗
       st ↦ {s with left' := s.left' - W64 1, next' := next, seen' := gasKeySeen s.seen' key.choice} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗ v.buf' ↦*{dq} xs ∗
       ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ gasFieldKeys.length⌝ ∗
       ⌜sint.Z (v.pos' + W64 key.bytes.length) = sint.Z v.pos' + key.bytes.length ∧
         sint.Z (v.pos' + W64 key.bytes.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 key.bytes.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  rcases key with ⟨bytes, choice, valid⟩
  dsimp only at fresh hbytes
  cases valid with
  | @integer head payload argument enc major bound =>
    wp_apply wp_nextField_integer c st fields v s f gasFieldKeys gasFieldNames xs payload rem head argument qf qo dq
      hfields hleft hstart gasFieldEncodings_nonempty gasFieldHeads_ordered gasCachedShortTable gasInlineFieldTable
      enc major bound hp hbytes $$ [Hc Hst Hfields Horder Hs] with ⟨Hc, Hst, Hfields, Horder, Hs, %Hnext⟩
    · iframe Hc Hst Hfields Horder Hs
    isimp only [GasKeyWire.choice, GasKeyWire.bytes, gasKeyCode] at HΦ
    iapply HΦ
    iexists s.next'
    isimp only [gasKeyCode, gasKeySeen, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
    iframe Hc Hst Hfields Horder Hnames Hs
    ipureintro
    refine ⟨hstart, ?_⟩
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext
  | @text head payload key choice enc major width ascii meaning =>
    wp_apply wp_nextField_ascii c st fields v s f gasFieldKeys gasFieldNames xs payload key rem head kind qf qo qn dq
      hfields hleft hstart gasFieldEncodings_nonempty gasFieldHeads_ordered gasFieldEncodings_apart gasCachedShortTable
      gasInlineFieldTable (by decide) enc major width ascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨%next, Hc, Hfields, Horder, Hnames, Hs, %Hnext, %Hsearch, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    have hnext := fieldSearchAdvance_bounds gasFieldKeys ((head :: payload) ++ key ++ rem) s.next' next hstart (by change (2 : Nat) < 2^63; decide) Hsearch
    have hlen : (((head :: payload) ++ key).length : Int) = 1 + payload.length + key.length := by simp; omega
    cases choice with
    | none =>
      ihave Hout := fieldReadOutcome_unknown st {s with left' := s.left' - W64 1, next' := next} gasFieldNames key code err kind
        (v.pos' + W64 (1 + payload.length + key.length)) meaning $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [GasKeyWire.choice, GasKeyWire.bytes, gasKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [gasKeyCode, gasKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext
    | some wei =>
      ihave Hout := fieldReadOutcome_known st {s with left' := s.left' - W64 1, next' := next} gasFieldNames key
        (gasPriceFieldName wei) (gasKeyIndex wei) code err kind (v.pos' + W64 (1 + payload.length + key.length))
        gasFoldedNames_distinct (by cases wei <;> decide) (gasKey_lookup wei) meaning $$ Hout
      ihave Hout := knownFieldOutcome_fresh st {s with left' := s.left' - W64 1, next' := next}
        (gasKeyIndex wei) (gasPriceFieldName wei) code err kind (v.pos' + W64 (1 + payload.length + key.length)) fresh $$ Hout
      icases Hout with ⟨%Heq, Hst⟩
      rcases Heq with ⟨rfl, rfl⟩
      isimp only [GasKeyWire.choice, GasKeyWire.bytes, gasKeyCode] at HΦ
      iapply HΦ
      iexists next
      isimp only [gasKeyCode, gasKeySeen, hlen]
      iframe Hc Hst Hfields Horder Hnames Hs
      ipureintro
      simpa only [hlen, Int.add_assoc] using And.intro hnext Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.gasKey_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextGasKey
