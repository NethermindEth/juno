import ProofCheck.FeltValueComplete
import ProofCheck.HeaderProjectionOwnership
import ProofCheck.TransactionInitialize

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

/-- The only stored application value is a four-limb felt. The other nineteen
    Go fields have the unique empty discardedCBOR representation. -/
def transactionHashLayout (value : GoArray w64 4) : transactionHashProjection :=
  {(zero_val transactionHashProjection) with TransactionHash' := value}

def TransactionHashOwned (dst : Loc) (value : GoArray w64 4) : IProp GF :=
  dst ↦ transactionHashLayout value

theorem transactionHashLayout_covers (h : transactionHashProjection) :
    h = transactionHashLayout h.TransactionHash' := by
  cases h
  unfold transactionHashLayout
  congr 1 <;> apply discardedCBOR_eq_zero

theorem transactionHashLayout_zero :
    transactionHashLayout (zero_val (GoArray w64 4)) = zero_val transactionHashProjection := rfl

/-- Borrow the by-value array, retaining all discarded field cells for exact
    reconstruction of the whole concrete destination after the helper returns. -/
theorem transactionHashField_acc (dst : Loc) (h : transactionHashProjection) :
    dst ↦ h ⊢ (iprop(structFieldRef transactionHashProjection go!"TransactionHash" dst ↦ h.TransactionHash' ∗
      (∀ value : GoArray w64 4, structFieldRef transactionHashProjection go!"TransactionHash" dst ↦ value -∗
        dst ↦ {h with TransactionHash' := value})) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hfield, Hrest⟩
  iframe Hfield
  iintro %value Hfield
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hfield Hrest

theorem transactionHashOwned_length (dst : Loc) (value : GoArray w64 4) :
    TransactionHashOwned (GF := GF) dst value ⊢ ⌜value.arr.length = 4⌝ := by
  isimp only [TransactionHashOwned]
  iintro Hdst
  icases transactionHashField_acc dst (transactionHashLayout value) $$ Hdst with ⟨Hfield, _⟩
  isimp only [transactionHashLayout] at Hfield
  ihave %Hlen := array_len (structFieldRef transactionHashProjection go!"TransactionHash" dst) (DFrac.own 1) 4 value.arr $$ Hfield
  ipureintro; omega

/-- Destructure all four owned words without assuming that malformed model
    arrays are valid Go values. The length comes from actual ownership. -/
theorem feltValue_four_words (value : GoArray w64 4) (length : value.arr.length = 4) :
    ∃ a b c d : w64, value = array.mk 4 [a, b, c, d] := by
  cases value with
  | mk words =>
    cases words with
    | nil => simp at length
    | cons a words =>
      cases words with
      | nil => simp at length
      | cons b words =>
        cases words with
        | nil => simp at length
        | cons c words =>
          cases words with
          | nil => simp at length
          | cons d words =>
            have empty : words = [] := by simpa using length
            subst words
            exact ⟨a, b, c, d, rfl⟩

/-- Borrow a single array word for the guard and restore the complete
    projection afterwards; no array equality or slice-index rule is used. -/
theorem wp_transactionHashWord (dst : Loc) (value : GoArray w64 4) (index : Int) (word : w64)
    (nonnegative : 0 ≤ index) (lookup : value.arr[index.toNat]? = some word) :
    {{ TransactionHashOwned (GF := GF) dst value }}
      (App (Val (GoInstruction (GoLoad go.uint64)))
        (Val #(arrayIndexRef w64 index (structFieldRef transactionHashProjection go!"TransactionHash" dst))))
    {{ RET #word; TransactionHashOwned dst value }} := by
  iintro %Φ Hdst HΦ
  isimp only [TransactionHashOwned] at Hdst
  icases transactionHashField_acc dst (transactionHashLayout value) $$ Hdst with ⟨Hfield, Hrestore⟩
  isimp only [transactionHashLayout] at Hfield
  icases array_acc (structFieldRef transactionHashProjection go!"TransactionHash" dst) index
    (DFrac.own 1) 4 value word nonnegative lookup $$ Hfield with ⟨Hword, Harray⟩
  wp_apply IntoValTyped.wp_load (t := go.uint64) _ _ _ $$ Hword as Hword
  ihave Hfield := Harray $$ %word Hword
  have same : value.arr.set index.toNat word = value.arr := list_set_lookup_self value.arr index.toNat word lookup
  isimp only [same] at Hfield
  ihave Hdst := Hrestore $$ %value Hfield
  iapply HΦ
  isimp only [TransactionHashOwned, transactionHashLayout]
  iexact Hdst

/-- The production guard compares each word with zero and preserves the entire
    destination. It does not require Go array equality or dereference a pointer. -/
theorem wp_zeroTransactionHash_owned (dst : Loc) (value : GoArray w64 4) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       TransactionHashOwned dst value }}
      (App (Val (@! cborZeroTransactionHashProjection)) (Val #dst))
    {{ (result : Bool), RET #result; TransactionHashOwned dst value ∗
       ⌜result = true ↔ value = zero_val (GoArray w64 4)⌝ }} := by
  wp_start as Hdst
  ihave %Hlen := transactionHashOwned_length dst value $$ Hdst
  obtain ⟨a, b, c, d, rfl⟩ := feltValue_four_words value Hlen
  wp_auto
  wp_apply wp_transactionHashWord dst (array.mk 4 [a, b, c, d]) (sint.Z (W64 0)) a (by decide) rfl $$ [Hdst] with Hdst
  · iframe Hdst
  cases ha : decide (a = W64 0) <;> cases hb : decide (b = W64 0) <;> cases hc : decide (c = W64 0)
  all_goals
    try simp only [ha, hb, hc, _root_.decide_true, _root_.decide_false]
    try wp_auto
  all_goals
    try
      wp_apply wp_transactionHashWord dst (array.mk 4 [a, b, c, d]) (sint.Z (W64 1)) b (by decide) rfl $$ [Hdst] with Hdst
      · iframe Hdst
    try simp only [ha, hb, hc, _root_.decide_true, _root_.decide_false]
    try wp_auto
  all_goals
    try
      wp_apply wp_transactionHashWord dst (array.mk 4 [a, b, c, d]) (sint.Z (W64 2)) c (by decide) rfl $$ [Hdst] with Hdst
      · iframe Hdst
    try simp only [ha, hb, hc, _root_.decide_true, _root_.decide_false]
    try wp_auto
  all_goals
    try
      wp_apply wp_transactionHashWord dst (array.mk 4 [a, b, c, d]) (sint.Z (W64 3)) d (by decide) rfl $$ [Hdst] with Hdst
      · iframe Hdst
    try simp only [ha, hb, hc, _root_.decide_true, _root_.decide_false]
    try wp_auto
  all_goals
    iapply HΦ
    iframe Hdst
    ipureintro
    simp_all [array.mk, show zero_val (GoArray w64 4) = array.mk 4 [W64 0, W64 0, W64 0, W64 0] from rfl]

/-- Actual whole-struct cleanup resets the four words and the empty skeleton. -/
theorem wp_clearTransactionHash_owned (dst : Loc) (value : GoArray w64 4) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       TransactionHashOwned dst value }}
      (App (Val (@! cborClearTransactionHashProjection)) (Val #dst))
    {{ RET #(); TransactionHashOwned dst (zero_val (GoArray w64 4)) }} := by
  wp_start as Hdst
  isimp only [TransactionHashOwned] at Hdst
  wp_auto
  iapply HΦ
  isimp only [TransactionHashOwned, transactionHashLayout_zero]
  iexact Hdst

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashLayout_covers
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashLayout_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashOwned_length
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValue_four_words
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroTransactionHash_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearTransactionHash_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transactionHashWord
