import ProofCheck.HeaderOwnership
import ProofCheck.ScalarComplete

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

/-- The four unsigned counts/timestamps and the 64-bit DA mode. -/
inductive HeaderScalarField where
  | number
  | transactions
  | events
  | timestamp
  | mode

def headerScalarName : HeaderScalarField → GoString
  | .number => go!"Number"
  | .transactions => go!"TransactionCount"
  | .events => go!"EventCount"
  | .timestamp => go!"Timestamp"
  | .mode => go!"L1DAMode"

def headerScalarWord (field : HeaderScalarField) (h : Header) : w64 :=
  match field with
  | .number => h.Number'
  | .transactions => h.TransactionCount'
  | .events => h.EventCount'
  | .timestamp => h.Timestamp'
  | .mode => h.L1DAMode'

def headerScalarValue (field : HeaderScalarField) (h : HeaderValue) : w64 :=
  match field with
  | .number => h.number
  | .transactions => h.transactionCount
  | .events => h.eventCount
  | .timestamp => h.timestamp
  | .mode => h.l1DAMode

def headerSetScalarWord (field : HeaderScalarField) (h : Header) (word : w64) : Header :=
  match field with
  | .number => {h with Number' := word}
  | .transactions => {h with TransactionCount' := word}
  | .events => {h with EventCount' := word}
  | .timestamp => {h with Timestamp' := word}
  | .mode => {h with L1DAMode' := word}

def headerSetScalarValue (field : HeaderScalarField) (old : HeaderValue) (word : w64) : HeaderValue :=
  match field with
  | .number => {old with number := word}
  | .transactions => {old with transactionCount := word}
  | .events => {old with eventCount := word}
  | .timestamp => {old with timestamp := word}
  | .mode => {old with l1DAMode := word}

def headerScalarMode : HeaderScalarField → Bool
  | .mode => true
  | _ => false

theorem headerSetScalarWord_original (field : HeaderScalarField) (h : Header) :
    headerSetScalarWord field h (headerScalarWord field h) = h := by cases field <;> rfl

theorem headerScalars_word (field : HeaderScalarField) (h : Header) (old : HeaderValue)
    (hs : HeaderScalars h old) : headerScalarWord field h = headerScalarValue field old := by
  rcases hs with ⟨hn, ht, he, htime, hv, hm⟩
  cases field <;> assumption

/-- Borrow exactly one scalar field from the actual fifteen-field record. -/
theorem headerScalar_acc (dst : Loc) (h : Header) (field : HeaderScalarField) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Header (headerScalarName field) dst ↦ headerScalarWord field h ∗
      (∀ word : w64, structFieldRef Header (headerScalarName field) dst ↦ word -∗ dst ↦ headerSetScalarWord field h word)) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup _ _ _ $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split _ _ _ $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hhash, Hparent, Hnumber, Hroot, Hsequencer, Htransactions, Hevents,
    Htime, Hversion, Heth, Hsignatures, Hstrk, Hmode, HdataGas, Hl2Gas, -⟩
  cases field <;> isimp only [headerScalarName, headerScalarWord, headerSetScalarWord]
  all_goals
    iframe
    iintro %word Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe

theorem headerBody_set_scalar (h : Header) (old : HeaderValue) (field : HeaderScalarField) (word : w64) :
    HeaderBody h old ⊢
      (HeaderBody (headerSetScalarWord field h word) (headerSetScalarValue field old word) : IProp GF) := by
  unfold HeaderBody
  iintro ⟨%Hscalars, Hhash, Hparent, Hroot, Hsequencer, Heth, Hsignatures, Hstrk⟩
  cases field <;> isimp only [headerSetScalarWord, headerSetScalarValue]
  all_goals
    iframe
    ipureintro
    rcases Hscalars with ⟨hn, ht, he, htime, hv, hm⟩
    simp_all [HeaderScalars]

theorem headerGasState_set_scalar (base : Loc) (h : Header) (old : HeaderValue)
    (field : HeaderScalarField) (word : w64) :
    HeaderGasState (GF := GF) base (headerSetScalarWord field h word) (headerSetScalarValue field old word) =
      HeaderGasState base h old := by cases field <;> rfl

/-- The field relation uses the independent old application value, so null and
    undefined retain that value even for a previously populated Header. -/
def HeaderScalarOutcome (c dst base : Loc) (h : Header) (old : HeaderValue) (field : HeaderScalarField)
    (v : cursor.Cursor) (xs wire : List w8) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : w64), dst ↦ headerSetScalarWord field h value ∗
    HeaderBody (headerSetScalarWord field h value) (headerSetScalarValue field old value) ∗
    HeaderGasState base (headerSetScalarWord field h value) (headerSetScalarValue field old value) ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ ScalarValid (headerScalarValue field old) item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (reason : ExpectedFailureReason) (err : Loc), dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗
    ExpectedFailureError err v.pos' (W8 0) reason (expectedFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ ScalarFailure wire reason⌝))

/-- All five actual scalar calls preserve the other fourteen fields, nested
    children and private shared allocation on arbitrary input. -/
theorem wp_headerScalar_complete (c dst base : Loc) (h : Header) (old : HeaderValue) (field : HeaderScalarField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       FeltErrorGlobals kinds ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasState base h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if headerScalarMode field then (@! cborReadMode) else (@! cborReadUint64))) (Val #c))
        (Val #(structFieldRef Header (headerScalarName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ HeaderScalarOutcome c dst base h old field v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hh, Hchildren, Hprices, Hc, Hs⟩ HΦ
  isimp only [HeaderBody] at Hchildren
  icases Hchildren with ⟨%Hscalars, Hchildren⟩
  ihave Hbody : HeaderBody h old $$ [Hchildren]
  · unfold HeaderBody
    iframe Hchildren
    ipureintro
    exact Hscalars
  have hword := headerScalars_word field h old Hscalars
  icases headerScalar_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readScalar_complete (headerScalarMode field) c (structFieldRef Header (headerScalarName field) dst)
    (headerScalarWord field h) v xs wire dq kinds hp hbytes $$ [Hglobals Hfield Hc Hs] as %result ⟨Hs, Hout⟩
  · iframe Hglobals Hfield Hc Hs
  iapply HΦ
  iframe Hs
  unfold ScalarOutcome HeaderScalarOutcome
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %value, Hfield, Hc, %Hnext⟩
    ihave Hh := Hclose $$ %value Hfield
    ihave Hbody := headerBody_set_scalar h old field value $$ Hbody
    ileft
    iexists item, rest, value
    isimp only [headerGasState_set_scalar]
    iframe Hh Hbody Hprices Hc
    ipureintro
    simpa only [hword] using Hnext
  · icases Hfailure with ⟨%reason, %err, Hfield, Hc, Herr, %Herror⟩
    ihave Hh := Hclose $$ %(headerScalarWord field h) Hfield
    isimp only [headerSetScalarWord_original] at Hh
    iright
    iexists reason, err
    iframe Hh Hbody Hprices Hc Herr
    ipureintro
    exact Herror

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSetScalarWord_original
#print axioms Perennial.github_com.NethermindEth.juno.core.headerScalars_word
#print axioms Perennial.github_com.NethermindEth.juno.core.headerScalar_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerBody_set_scalar
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_set_scalar
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerScalar_complete
