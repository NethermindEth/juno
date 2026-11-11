import ProofCheck.HeaderProjectionOwnership
import ProofCheck.FeltFailureSound

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

/-- All four concrete felt slots in the five Header projection layouts. -/
inductive HeaderProjectionFeltSlot where
  | hash | root | pairHash | pairRoot

abbrev headerProjectionFeltKind : HeaderProjectionFeltSlot → HeaderProjectionKind
  | .hash => .hash
  | .root => .root
  | .pairHash => .hashAndRoot
  | .pairRoot => .hashAndRoot

def headerProjectionFeltField : HeaderProjectionFeltSlot → HeaderProjectionField
  | .hash => .hash
  | .root => .root
  | .pairHash => .hash
  | .pairRoot => .root

def headerProjectionFeltRef (slot : HeaderProjectionFeltSlot) (dst : Loc) : Loc :=
  match slot with
  | .hash => structFieldRef headerHashProjection go!"Hash" dst
  | .root => structFieldRef headerGlobalStateRootProjection go!"GlobalStateRoot" dst
  | .pairHash => structFieldRef headerHashAndStateRootProjection go!"Hash" dst
  | .pairRoot => structFieldRef headerHashAndStateRootProjection go!"GlobalStateRoot" dst

def headerProjectionFeltPointer (slot : HeaderProjectionFeltSlot) : HeaderProjectionLayout (headerProjectionFeltKind slot) → Loc :=
  match slot with
  | .hash => fun h => h.Hash'
  | .root => fun h => h.GlobalStateRoot'
  | .pairHash => fun h => h.Hash'
  | .pairRoot => fun h => h.GlobalStateRoot'

def headerProjectionSetFeltPointer (slot : HeaderProjectionFeltSlot) (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) (p : Loc) : HeaderProjectionLayout (headerProjectionFeltKind slot) :=
  match slot, h with
  | .hash, h => {h with Hash' := p}
  | .root, h => {h with GlobalStateRoot' := p}
  | .pairHash, h => {h with Hash' := p}
  | .pairRoot, h => {h with GlobalStateRoot' := p}

def headerProjectionSetFeltValue (slot : HeaderProjectionFeltSlot) (old : HeaderProjectionValue) (value : Option (GoArray w64 4)) : HeaderProjectionValue :=
  match slot with
  | .hash => {old with hash := value}
  | .root => {old with root := value}
  | .pairHash => {old with hash := value}
  | .pairRoot => {old with root := value}

theorem headerProjectionSetFeltPointer_original (slot : HeaderProjectionFeltSlot)
    (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) :
    headerProjectionSetFeltPointer slot h (headerProjectionFeltPointer slot h) = h := by
  cases slot <;> rfl

/-- Borrow exactly the selected pointer cell and reconstruct the actual layout
    after any replacement, preserving the embedded skeleton and other slot. -/
theorem headerProjectionFelt_acc (slot : HeaderProjectionFeltSlot) (dst : Loc)
    (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) :
    HeaderProjectionCells (headerProjectionFeltKind slot) dst h ⊢
      (headerProjectionFeltRef (GF := GF) slot dst ↦ headerProjectionFeltPointer slot h ∗
       (∀ p : Loc, headerProjectionFeltRef (GF := GF) slot dst ↦ p -∗
         HeaderProjectionCells (headerProjectionFeltKind slot) dst (headerProjectionSetFeltPointer slot h p)) : IProp GF) := by
  cases slot <;> simp only [headerProjectionFeltKind, HeaderProjectionCells, headerProjectionFeltRef,
    headerProjectionFeltPointer, headerProjectionSetFeltPointer]
  all_goals
    iintro Hh
    icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
    icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
    isimp only [TypedPointsto.typedPointstoDef] at Hh
    first
    | icases Hh with ⟨Hskeleton, Hhash, Hroot, -⟩
    | icases Hh with ⟨Hskeleton, Hfield, -⟩
    iframe
    iintro %p Hvalue
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe

/-- Publishing one newly owned felt preserves every other application slot. -/
theorem headerProjectionChildren_set_felt (slot : HeaderProjectionFeltSlot)
    (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) (old : HeaderProjectionValue)
    (p : Loc) (value : Option (GoArray w64 4)) :
    HeaderProjectionChildren (headerProjectionFeltKind slot) h old ∗ FeltResult p value ⊢
      (HeaderProjectionChildren (headerProjectionFeltKind slot) (headerProjectionSetFeltPointer slot h p)
        (headerProjectionSetFeltValue slot old value) : IProp GF) := by
  cases slot <;> simp only [headerProjectionFeltKind, HeaderProjectionChildren,
    headerProjectionSetFeltPointer, headerProjectionSetFeltValue]
  all_goals
    iintro ⟨⟨%Hcanonical, Hchildren⟩, Hvalue⟩
    isplitl []
    · ipureintro; exact Hcanonical
    first
    | iexact Hvalue
    | icases Hchildren with ⟨Hhash, Hroot⟩; iframe

/-- The concrete slot mapping realizes the independent application field. -/
theorem headerProjectionFelt_field_valid (d : Nat) (slot : HeaderProjectionFeltSlot)
    (old : HeaderProjectionValue) (wire : List w8) (value : Option (GoArray w64 4))
    (valid : FeltValid d wire value) :
    HeaderProjectionFieldValid d (headerProjectionFeltField slot) old wire (headerProjectionSetFeltValue slot old value) := by
  cases slot <;> first | exact .hash valid | exact .root valid

/-- Actual felt helper calls at all four projection slots, including fresh
    allocation, exact suffixes and preservation of all unrelated ownership. -/
theorem wp_headerProjectionFelt_valid (slot : HeaderProjectionFeltSlot) (c dst : Loc)
    (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) (old : HeaderProjectionValue)
    (v : cursor.Cursor) (xs wire rem : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32) (valid : FeltValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       HeaderProjectionCells (headerProjectionFeltKind slot) dst h ∗
       HeaderProjectionChildren (headerProjectionFeltKind slot) h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(headerProjectionFeltRef (GF := GF) slot dst)))
    {{ (p : Loc), RET #interface.nil;
       HeaderProjectionCells (headerProjectionFeltKind slot) dst (headerProjectionSetFeltPointer slot h p) ∗
       HeaderProjectionChildren (headerProjectionFeltKind slot) (headerProjectionSetFeltPointer slot h p)
         (headerProjectionSetFeltValue slot old value) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hh, Hchildren, Hc, Hs⟩
  icases headerProjectionFelt_acc slot dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_valid c (headerProjectionFeltRef (GF := GF) slot dst) (headerProjectionFeltPointer slot h)
    v xs wire rem value dq d hd depth valid hp hbytes $$ [Hfield Hc Hs] as %p ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %p Hfield
  ihave Hchildren := headerProjectionChildren_set_felt slot h old p value $$ [Hchildren Hvalue]
  · iframe Hchildren Hvalue
  wp_end

/-- Any selected felt failure preserves the whole concrete projection and its
    previous child ownership, with exact bounded diagnostic position. -/
theorem wp_headerProjectionFelt_failure (slot : HeaderProjectionFeltSlot) (c dst : Loc)
    (h : HeaderProjectionLayout (headerProjectionFeltKind slot)) (old : HeaderProjectionValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : FeltFailure d wire offset reason) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ HeaderProjectionCells (headerProjectionFeltKind slot) dst h ∗
       HeaderProjectionChildren (headerProjectionFeltKind slot) h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(headerProjectionFeltRef (GF := GF) slot dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionCells (headerProjectionFeltKind slot) dst h ∗ HeaderProjectionChildren (headerProjectionFeltKind slot) h old ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧ sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hchildren, Hc, Hs⟩
  icases headerProjectionFelt_acc slot dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readFelt_failure_bounded c (headerProjectionFeltRef (GF := GF) slot dst) (headerProjectionFeltPointer slot h)
    v xs wire dq d offset reason kind failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hoffset⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %(headerProjectionFeltPointer slot h) Hfield
  isimp only [headerProjectionSetFeltPointer_original] at Hh
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionSetFeltPointer_original
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFelt_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionChildren_set_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFelt_field_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionFelt_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionFelt_failure
