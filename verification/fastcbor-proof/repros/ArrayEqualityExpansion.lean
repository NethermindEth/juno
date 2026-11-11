-- Review aid only: the proposed RHS, with no new step rule or trusted assumption.
-- Outside ProofCheck. These checks do not prove that Go array equality steps here.
import Perennial.Golang.Theory.Map
import ProofCheck.GeneratedProof.github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option linter.unusedSimpArgs false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial.ArrayEqualityProposal
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def expansion (n : Int) (elem_type : go.GoType) {V : Type} (v1 v2 : GoArray V n) : Expr :=
  if ¬(0 ≤ n ∧ n < 2^63 ∧ (v1.arr.length : Int) = n ∧ (v2.arr.length : Int) = n) then
    gl(AngelicExit #())
  else
    List.foldl (fun cmp_so_far (j : Nat) =>
      gl(if: cmp_so_far then
        (Index (go.ArrayType n elem_type) (#v1, #(W64 (j : Int)))) =⟨elem_type⟩
        (Index (go.ArrayType n elem_type) (#v2, #(W64 (j : Int))))
      else #false))
      (#true : Expr) (List.range n.toNat)

example (elem_type : go.GoType) (V : Type) :
    expansion 0 elem_type (array.mk 0 ([] : List V)) (array.mk 0 []) = #true := by
  simp [expansion]

example (elem_type : go.GoType) (V : Type) (v1 v2 : GoArray V 4)
    (bad : v1.arr.length ≠ 4) :
    expansion 4 elem_type v1 v2 = gl(AngelicExit #()) := by
  have bad' : (v1.arr.length : Int) ≠ 4 := by omega
  simp [expansion, bad']

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]

-- Four arbitrary words; the comparison executes the existing uint64 equality.
theorem wp_four_words (a b c d e f g h : w64) :
    {{ (True : IProp GF) }}
      (expansion 4 go.uint64 (array.mk 4 [a, b, c, d]) (array.mk 4 [e, f, g, h]))
    {{ RET #(decide (a = e ∧ b = f ∧ c = g ∧ d = h)); (True : IProp GF) }} := by
  have hret : decide (a = e ∧ b = f ∧ c = g ∧ d = h) =
      (decide (a = e) && decide (b = f) && decide (c = g) && decide (d = h)) := by
    by_cases ha : a = e <;> by_cases hb : b = f <;> by_cases hc : c = g <;>
      by_cases hd : d = h <;> simp_all
  rw [hret]
  simp only [expansion]
  simp only [List.length_cons, List.length_nil, Int.reduceToNat, Nat.reduceAdd,
    Int.reduceLE, Int.reduceLT, Int.reducePow, _root_.and_self,
    List.range_succ, List.range_zero, List.foldl_append, List.foldl_cons,
    List.foldl_nil]
  wp_start
  wp_auto
  cases ha : decide (a = e) <;> cases hb : decide (b = f) <;> cases hc : decide (c = g)
  all_goals
    try simp only [ha, hb, hc]
    wp_auto
    try simp only [ha, hb, hc]
    try wp_auto
    try simp only [ha, hb, hc]
    try wp_auto
    isimp only [ha, hb, hc, Bool.false_and, Bool.true_and] at HΦ
    iapply HΦ
    iempintro

-- The second pair may contain uncomparable dynamic values. It is never reached.
theorem wp_interface_short_circuit (leftTail rightTail : GoInterface) :
    {{ (True : IProp GF) }}
      (expansion 2 (go.InterfaceType [])
        (array.mk 2 [interface.mkOk go.uint64 #(W64 0), leftTail])
        (array.mk 2 [interface.mkOk go.uint64 #(W64 1), rightTail]))
    {{ RET #false; (True : IProp GF) }} := by
  simp only [expansion]
  simp only [List.length_cons, List.length_nil, Int.reduceToNat, Nat.reduceAdd,
    Int.reduceLE, Int.reduceLT, Int.reducePow, _root_.and_self,
    List.range_succ, List.range_zero, List.foldl_append, List.foldl_cons, List.foldl_nil]
  wp_start
  wp_auto
  iapply HΦ
  iempintro

-- Float words are compared by the existing float operation, not bit equality.
theorem wp_float_delegation (left right : w64) :
    {{ (True : IProp GF) }}
      (expansion 1 go.float64 (array.mk 1 [left]) (array.mk 1 [right]))
    {{ RET #(float64Eq left right); (True : IProp GF) }} := by
  simp only [expansion]
  simp only [List.length_cons, List.length_nil, Int.reduceToNat, Nat.reduceAdd,
    Int.reduceLE, Int.reduceLT, Int.reducePow, _root_.and_self,
    List.range_succ, List.range_zero, List.foldl_append, List.foldl_cons, List.foldl_nil]
  wp_start
  wp_auto
  iapply HΦ
  iempintro

end Perennial.ArrayEqualityProposal

#print axioms Perennial.ArrayEqualityProposal.wp_four_words
#print axioms Perennial.ArrayEqualityProposal.wp_interface_short_circuit
#print axioms Perennial.ArrayEqualityProposal.wp_float_delegation
