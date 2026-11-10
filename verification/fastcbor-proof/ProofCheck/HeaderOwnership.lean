import ProofCheck.GasPriceFailureSlot

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

/-- Nullable GasPrice output, independent of the shared allocation strategy. -/
def GasPriceResult (p : Loc) (value : Option GasPriceValue) : IProp GF :=
  match value with
  | none => iprop(⌜p = null⌝)
  | some value => GasPriceOwned p value

/-- Split the shared two-element allocation into the disjoint outputs exposed
    by Header. Neither struct nor either nested felt ownership is duplicated. -/
theorem gasPrices_owned_pair (base : Loc) (prices : GoArray GasPrice 2) (first second : GasPriceValue) :
    GasPricesOwned base prices [first, second] ⊢
      (GasPriceOwned (arrayIndexRef GasPrice 0 base) first ∗
       GasPriceOwned (arrayIndexRef GasPrice 1 base) second : IProp GF) := by
  unfold GasPricesOwned
  iintro ⟨Hprices, Hchildren⟩
  ihave %Hlen := BigSepL2.bigSepL2_length $$ Hchildren
  rcases prices with ⟨items⟩
  cases items with
  | nil => simp at Hlen
  | cons g0 tail =>
    cases tail with
    | nil => simp at Hlen
    | cons g1 tail =>
      have htail : tail = [] := by
        cases tail with
        | nil => rfl
        | cons x xs => simp only [List.length_cons, List.length_nil] at Hlen; omega
      subst tail
      icases typedPointsto_split _ _ _ $$ Hprices with Hprices
      isimp only [TypedPointsto.typedPointstoDef] at Hprices
      icases Hprices with ⟨-, Hcells⟩
      isimp only [arrayElems, BigSepL.bigSepL_cons, BigSepL.bigSepL_nil] at Hcells
      icases Hcells with ⟨Hg0, Hg1, -⟩
      icases BigSepL2.bigSepL2_cons.1 $$ Hchildren with ⟨Hfirst, Hchildren⟩
      icases BigSepL2.bigSepL2_cons.1 $$ Hchildren with ⟨Hsecond, -⟩
      unfold GasPriceOwned
      isplitl [Hg0 Hfirst]
      · iexists g0
        iframe Hg0 Hfirst
      · iexists g1
        iframe Hg1 Hsecond

/-- Unpublished slots may be discarded; published slots retain separate owned
    struct/felt contents. Null pointers remain distinct from zero-valued structs. -/
theorem gasPrices_owned_results (base : Loc) (prices : GoArray GasPrice 2)
    (first second : GasPriceValue) (publishFirst publishSecond : Bool) :
    GasPricesOwned base prices [first, second] ⊢
      (GasPriceResult (if publishFirst then arrayIndexRef GasPrice 0 base else null)
         (if publishFirst then some first else none) ∗
       GasPriceResult (if publishSecond then arrayIndexRef GasPrice 1 base else null)
         (if publishSecond then some second else none) : IProp GF) := by
  iintro Hprices
  icases gasPrices_owned_pair base prices first second $$ Hprices with ⟨Hfirst, Hsecond⟩
  cases publishFirst <;> cases publishSecond <;>
    isimp only [Bool.false_eq_true, ite_false, ite_true, GasPriceResult]
  all_goals iframe
  all_goals ipureintro
  all_goals simp

/-- All fifteen application fields, with values independent of heap addresses.
    The nested option/list shape preserves every nil/empty distinction. Scalars
    use the pinned 64-bit target's words; strings retain their exact byte value. -/
structure HeaderValue where
  hash : Option (GoArray w64 4)
  parentHash : Option (GoArray w64 4)
  number : w64
  globalStateRoot : Option (GoArray w64 4)
  sequencerAddress : Option (GoArray w64 4)
  transactionCount : w64
  eventCount : w64
  timestamp : w64
  protocolVersion : GoString
  l1GasPriceETH : Option (GoArray w64 4)
  signatures : Option (List SignatureValue)
  l1GasPriceSTRK : Option (GoArray w64 4)
  l1DAMode : w64
  l1DataGasPrice : Option GasPriceValue
  l2GasPrice : Option GasPriceValue

def headerZeroValue : HeaderValue :=
  ⟨none, none, W64 0, none, none, W64 0, W64 0, W64 0, [], none, none, none, W64 0, none, none⟩

def HeaderScalars (h : Header) (value : HeaderValue) : Prop :=
  h.Number' = value.number ∧ h.TransactionCount' = value.transactionCount ∧
  h.EventCount' = value.eventCount ∧ h.Timestamp' = value.timestamp ∧
  h.ProtocolVersion' = value.protocolVersion ∧ h.L1DAMode' = value.l1DAMode

/-- Fields independent of the decoder's shared GasPrice allocation. -/
def HeaderBody (h : Header) (value : HeaderValue) : IProp GF := iprop(
  ⌜HeaderScalars h value⌝ ∗ FeltResult h.Hash' value.hash ∗
  FeltResult h.ParentHash' value.parentHash ∗ FeltResult h.GlobalStateRoot' value.globalStateRoot ∗
  FeltResult h.SequencerAddress' value.sequencerAddress ∗ FeltResult h.L1GasPriceETH' value.l1GasPriceETH ∗
  SignaturesResult h.Signatures' value.signatures ∗ FeltResult h.L1GasPriceSTRK' value.l1GasPriceSTRK)

/-- Caller-visible output ownership, including both GasPrice children. -/
def HeaderChildren (h : Header) (value : HeaderValue) : IProp GF := iprop(
  HeaderBody h value ∗ GasPriceResult h.L1DataGasPrice' value.l1DataGasPrice ∗
  GasPriceResult h.L2GasPrice' value.l2GasPrice)

def HeaderOwned (dst : Loc) (value : HeaderValue) : IProp GF :=
  iprop(∃ h : Header, dst ↦ h ∗ HeaderChildren h value)

/-- Internal loop ownership before or after the shared pair is allocated. The
    publication flags are independent, so missing/null fields need no allocation. -/
def HeaderGasState (base : Loc) (h : Header) (value : HeaderValue) : IProp GF := iprop(
  (⌜base = null ∧ h.L1DataGasPrice' = null ∧ h.L2GasPrice' = null ∧
     value.l1DataGasPrice = none ∧ value.l2GasPrice = none⌝) ∨
  (∃ (prices : GoArray GasPrice 2) (first second : GasPriceValue) (publishFirst publishSecond : Bool),
    GasPricesOwned base prices [first, second] ∗
    ⌜h.L1DataGasPrice' = (if publishFirst then arrayIndexRef GasPrice 0 base else null) ∧
      h.L2GasPrice' = (if publishSecond then arrayIndexRef GasPrice 1 base else null) ∧
      value.l1DataGasPrice = (if publishFirst then some first else none) ∧
      value.l2GasPrice = (if publishSecond then some second else none)⌝))

/-- An untouched Header owns no nested allocations. -/
theorem headerChildren_zero : ⊢ (HeaderChildren (zero_val Header) headerZeroValue : IProp GF) := by
  unfold HeaderChildren HeaderBody HeaderScalars headerZeroValue FeltResult SignaturesResult GasPriceResult
  ipureintro
  exact ⟨⟨⟨rfl, rfl, rfl, rfl, rfl, rfl⟩, rfl, rfl, rfl, rfl, rfl, rfl, rfl⟩, rfl, rfl⟩

theorem headerGasState_zero : ⊢ (HeaderGasState null (zero_val Header) headerZeroValue : IProp GF) := by
  unfold HeaderGasState
  ileft
  ipureintro
  exact ⟨rfl, rfl, rfl, rfl, rfl⟩

/-- Discharge the private shared-allocation invariant at a successful return. -/
theorem headerGasState_publish (base : Loc) (h : Header) (value : HeaderValue) :
    HeaderBody h value ∗ HeaderGasState base h value ⊢ (HeaderChildren h value : IProp GF) := by
  iintro ⟨Hbody, Hprices⟩
  unfold HeaderChildren HeaderGasState
  iframe Hbody
  icases Hprices with (Hnil | Hshared)
  · icases Hnil with %Hnil
    rcases Hnil with ⟨_, hfirst, hsecond, vfirst, vsecond⟩
    isimp only [hfirst, hsecond, vfirst, vsecond, GasPriceResult]
    ipureintro
    trivial
  · icases Hshared with ⟨%prices, %first, %second, %publishFirst, %publishSecond, Hprices, %Hpublished⟩
    rcases Hpublished with ⟨hfirst, hsecond, vfirst, vsecond⟩
    isimp only [hfirst, hsecond, vfirst, vsecond]
    iapply gasPrices_owned_results base prices first second publishFirst publishSecond
    iexact Hprices

/-- The production cleanup resets every field for any partially populated
    Header. Child resources can be discarded under the GC/affine model; this
    proves reset and ownership of the zero result, not physical reclamation. -/
theorem wp_clearHeader_owned (dst : Loc) (h : Header) (value : HeaderValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗ dst ↦ h ∗ HeaderChildren h value }}
      (App (Val (@! cborClearHeader)) (Val #dst))
    {{ RET #(); HeaderOwned dst headerZeroValue }} := by
  wp_start_folded as ⟨Hdst, Hchildren⟩
  wp_apply wp_clearHeader dst h $$ [Hdst] with Hdst
  · iframe Hdst
  iapply HΦ
  unfold HeaderOwned
  iexists (zero_val Header)
  iframe Hdst
  iapply headerChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrices_owned_pair
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPrices_owned_results
#print axioms Perennial.github_com.NethermindEth.juno.core.headerChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasState_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeader_owned
