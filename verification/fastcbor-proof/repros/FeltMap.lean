-- Expected failure: the pinned model has no array-value equality rule.
-- Outside the audited target; no SafeMapKey premise is added to the proof.
import ProofCheck.GeneratedProof.github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps
import Perennial.Golang.Theory.Map

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]
variable [package_sem : github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps.Assumptions]

open github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps

/-- Actual translated Go witness: all felt keys, including collisions and misses.
    This is a map-operation feasibility result, not a CBOR decoding contract. -/
theorem wp_feltMap_witness (k1 k2 query : GoArray w64 4) (v1 v2 : Loc) :
    {{ (True : IProp GF) }}
      (App (App (App (App (App (Val (@! FeltEntries))
        (Val #k1)) (Val #k2)) (Val #query)) (Val #v1)) (Val #v2))
    {{ (m : Loc), RET (PairV (PairV #m #(((<[k2 := v2]> (<[k1 := v1]> ∅) : GMap (GoArray w64 4) Loc) !! query).getD null))
        #(decide (((<[k2 := v2]> (<[k1 := v1]> ∅) : GMap (GoArray w64 4) Loc) !! query).isSome)));
       m ↦$ (<[k2 := v2]> (<[k1 := v1]> ∅) : GMap (GoArray w64 4) Loc) }} := by
  wp_start
  wp_auto
  wp_apply (wp_map_make1 (K := GoArray w64 4) (V := Loc)) with %m Hm
  wp_apply wp_mapInsert $$ Hm with Hm
  wp_apply wp_mapInsert $$ Hm with Hm
  wp_apply wp_map_lookup2 $$ Hm with Hm
  isimp only [Bool.decide_coe] at HΦ
  isimp only [Bool.decide_coe]
  isimp only [GMap.lookup_insert_eq_iff, GMap.lookup_empty] at HΦ
  iapply HΦ $$ %m Hm

end github_com.NethermindEth.juno.core
end Perennial
