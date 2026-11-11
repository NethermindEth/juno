import ProofCheck.HeaderProjectionCanonical
import ProofCheck.HeaderOwnedZero
import ProofCheck.Uint64PointerComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The proof uses the actual five translated Go layouts. No replacement
    runtime representation is introduced. -/
abbrev HeaderProjectionLayout : HeaderProjectionKind → Type
  | .hash => headerHashProjection
  | .root => headerGlobalStateRootProjection
  | .count => headerTransactionCountProjection
  | .timestamp => headerTimestampProjection
  | .hashAndRoot => headerHashAndStateRootProjection

def headerProjectionLayoutZero (kind : HeaderProjectionKind) : HeaderProjectionLayout kind :=
  match kind with
  | .hash => zero_val headerHashProjection
  | .root => zero_val headerGlobalStateRootProjection
  | .count => zero_val headerTransactionCountProjection
  | .timestamp => zero_val headerTimestampProjection
  | .hashAndRoot => zero_val headerHashAndStateRootProjection

def HeaderProjectionCells (kind : HeaderProjectionKind) (dst : Loc) : HeaderProjectionLayout kind → IProp GF :=
  match kind with
  | .hash => fun h => dst ↦ h
  | .root => fun h => dst ↦ h
  | .count => fun h => dst ↦ h
  | .timestamp => fun h => dst ↦ h
  | .hashAndRoot => fun h => dst ↦ h

/-- Canonical absent slots and ownership of every published child. The paired
    projection uses separating conjunction, so its felt storage is disjoint. -/
def HeaderProjectionChildren (kind : HeaderProjectionKind) (h : HeaderProjectionLayout kind)
    (value : HeaderProjectionValue) : IProp GF := iprop(
  ⌜HeaderProjectionCanonical kind value⌝ ∗
  (match kind, h with
    | .hash, h => FeltResult h.Hash' value.hash
    | .root, h => FeltResult h.GlobalStateRoot' value.root
    | .count, h => iprop(⌜h.TransactionCount' = value.count⌝)
    | .timestamp, h => Uint64PointerResult h.Timestamp' value.timestamp
    | .hashAndRoot, h => iprop(FeltResult h.Hash' value.hash ∗ FeltResult h.GlobalStateRoot' value.root)))

def HeaderProjectionOwned (kind : HeaderProjectionKind) (dst : Loc) (value : HeaderProjectionValue) : IProp GF :=
  iprop(∃ h : HeaderProjectionLayout kind, HeaderProjectionCells kind dst h ∗ HeaderProjectionChildren kind h value)

theorem discardedCBOR_eq_zero (value : discardedCBOR) : value = zero_val discardedCBOR := by
  cases value
  rfl

/-- Embedded discarded fields carry no state, even for an arbitrary initial
    destination. Their ownership remains part of the concrete struct cell. -/
theorem discardedHeaderSkeleton_eq_zero (value : discardedHeaderSkeleton) : value = zero_val discardedHeaderSkeleton := by
  cases value
  congr 1 <;> apply discardedCBOR_eq_zero

theorem headerProjectionChildren_canonical (kind : HeaderProjectionKind)
    (h : HeaderProjectionLayout kind) (value : HeaderProjectionValue) :
    HeaderProjectionChildren (GF := GF) kind h value ⊢ ⌜HeaderProjectionCanonical kind value⌝ := by
  unfold HeaderProjectionChildren
  iintro ⟨%Hcanonical, _⟩
  ipureintro
  exact Hcanonical

theorem headerProjectionChildren_zero (kind : HeaderProjectionKind) :
    ⊢ (HeaderProjectionChildren (GF := GF) kind (headerProjectionLayoutZero kind) headerProjectionZero) := by
  cases kind <;> unfold HeaderProjectionChildren
  all_goals
    isplitr
    · ipureintro; exact headerProjectionZero_canonical _
    simp only [headerProjectionLayoutZero, headerProjectionZero, FeltResult, Uint64PointerResult]
    ipureintro
    first | rfl | exact ⟨rfl, rfl⟩

/-- Nil and allocated zero remain distinct. Under the ownership relation, each
    entire concrete layout is zero exactly when its application value is zero. -/
theorem headerProjectionChildren_zero_iff (kind : HeaderProjectionKind)
    (h : HeaderProjectionLayout kind) (value : HeaderProjectionValue) :
    HeaderProjectionChildren (GF := GF) kind h value ⊢
      ⌜h = headerProjectionLayoutZero kind ↔ value = headerProjectionZero⌝ := by
  cases kind with
  | hash =>
    unfold HeaderProjectionChildren
    iintro ⟨%Hcanonical, Hhash⟩
    ihave %Hhash := feltResult_null_iff h.Hash' value.hash $$ Hhash
    ipureintro
    cases h with
    | mk skeleton hash =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      cases value
      simp_all [HeaderProjectionCanonical, headerProjectionZero, headerProjectionLayoutZero]
      change headerHashProjection.mk _ hash = headerHashProjection.mk _ null ↔ _
      simpa only [headerHashProjection.mk.injEq, true_and] using Hhash
  | root =>
    unfold HeaderProjectionChildren
    iintro ⟨%Hcanonical, Hroot⟩
    ihave %Hroot := feltResult_null_iff h.GlobalStateRoot' value.root $$ Hroot
    ipureintro
    cases h with
    | mk skeleton root =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      cases value
      simp_all [HeaderProjectionCanonical, headerProjectionZero, headerProjectionLayoutZero]
      change headerGlobalStateRootProjection.mk _ root = headerGlobalStateRootProjection.mk _ null ↔ _
      simpa only [headerGlobalStateRootProjection.mk.injEq, true_and] using Hroot
  | count =>
    unfold HeaderProjectionChildren
    iintro ⟨%Hcanonical, %Hcount⟩
    ipureintro
    cases h with
    | mk skeleton count =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      cases value
      simp_all [HeaderProjectionCanonical, headerProjectionZero, headerProjectionLayoutZero]
      change headerTransactionCountProjection.mk _ _ = headerTransactionCountProjection.mk _ (W64 0) ↔ _
      simp
  | timestamp =>
    unfold HeaderProjectionChildren
    iintro ⟨%Hcanonical, Htimestamp⟩
    ihave %Htimestamp := uint64PointerResult_null_iff h.Timestamp' value.timestamp $$ Htimestamp
    ipureintro
    cases h with
    | mk skeleton timestamp =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      cases value
      simp_all [HeaderProjectionCanonical, headerProjectionZero, headerProjectionLayoutZero]
      change headerTimestampProjection.mk _ timestamp = headerTimestampProjection.mk _ null ↔ _
      simpa only [headerTimestampProjection.mk.injEq, true_and] using Htimestamp
  | hashAndRoot =>
    unfold HeaderProjectionChildren
    iintro ⟨%Hcanonical, Hhash, Hroot⟩
    ihave %Hhash := feltResult_null_iff h.Hash' value.hash $$ Hhash
    ihave %Hroot := feltResult_null_iff h.GlobalStateRoot' value.root $$ Hroot
    ipureintro
    cases h with
    | mk skeleton hash root =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      cases value
      simp_all [HeaderProjectionCanonical, headerProjectionZero, headerProjectionLayoutZero]
      change headerHashAndStateRootProjection.mk _ hash root = headerHashAndStateRootProjection.mk _ null null ↔ _
      simpa only [headerHashAndStateRootProjection.mk.injEq, true_and] using and_congr Hhash Hroot

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.discardedCBOR_eq_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.discardedHeaderSkeleton_eq_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_zero_iff
