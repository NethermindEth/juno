import ProofCheck.TransactionInitialize
import ProofCheck.WantedComplete

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

/-- An independently specified successful scan on an initialized transaction-package
    table, with exact consumption and state updates. This specializes the actual
    NextWanted proof and retains its explicit UTF-8 validator dependency. -/
theorem wp_nextWanted_transaction_valid (validator : Utf8ValidatorContract (GF := GF))
    (schema : TransactionSchema) (c st : Loc) (v : cursor.Cursor) (s : cursor.Struct)
    (seen : List Nat) (xs item rem : List w8) (dq : DFrac) (d n : Nat) (result : WantedResult)
    (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (hs : WantedSeen (transactionNames schema).length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (valid : WantedValid d (transactionNames schema) seen n item result)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       globalAddr (transactionGlobal schema) ↦□ s.fields' ∗ c ↦ v ∗ st ↦ s ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ RET (PairV #(wantedCode result) #interface.nil);
       c ↦ wantedCursor v s.open' (v.pos' + W64 item.length) result ∗ st ↦ wantedStruct s result ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨#Hglobal, Hc, Hst, Hs⟩
  ihave #Htable := transaction_initialized_table (GF := GF) schema $$ []
  · solve_pkg_init
  isimp only [TransactionTable] at Htable
  icases Htable with ⟨%fields, #Hpublished, #Htable⟩
  ihave %agree := typedPointsto_agree_wand (globalAddr (transactionGlobal schema))
    DFrac.discard DFrac.discard fields s.fields' $$ Hpublished Hglobal
  subst fields
  isimp only [StaticFieldsShared] at Htable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_apply wp_nextWanted_valid validator c st s.fields' v s f (transactionNames schema) seen xs item rem
    DFrac.discard DFrac.discard dq d n result rfl hleft countBound
    (transactionNames_small schema) (transactionNames_distinct schema) Hmask hs hd depth valid hp hbytes
    $$ [Hc Hst Hs] with ⟨Hc, Hst, _, _, Hs, %Hnext⟩
  · iframe Hc Hst Hfields Hnames Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_transaction_valid
