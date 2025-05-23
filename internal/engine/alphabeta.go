package engine

import (
	"math"
	"time"

	"danielyang.cc/chess/internal/board"
)

var searchStart time.Time

func alphaBeta(alpha, beta, depth int) (int, int) {
	if timeForMove > 0 && time.Since(searchStart).Milliseconds() >= int64(timeForMove) {
		return 0, 0
	}

	nodes++
	ply++
	defer func() { ply-- }()

	if ply != 0 && board.IsRepetition() || board.Fifty >= 100 {
		return 0, 0
	}

	// pv node
	pv := beta-alpha > 1

	// tt entry
	ttEntry, found := board.GetTTEntry()
	if ply != 0 && !pv && found && ttEntry.Depth >= depth {
		switch ttEntry.Type {
		case board.PVNode:
			return ttEntry.Move, ttEntry.Score
		case board.CutNode:
			if ttEntry.Score >= beta {
				return ttEntry.Move, ttEntry.Score
			}
		case board.AllNode:
			if ttEntry.Score <= alpha {
				return ttEntry.Move, ttEntry.Score
			}
		}
	}

	inCheck := board.InCheck()

	// increase depth in check
	if inCheck {
		depth++
	}

	// quiesce
	if depth <= 0 {
		return 0, quiesce(alpha, beta)
	}

	// null move pruning
	if depth >= 3 && ply != 0 && !inCheck {
		ply++

		board.MakeNullMove()

		// reduction factor = 2
		_, nullEval := alphaBeta(-beta, -beta+1, depth-1-2)
		nullEval *= -1

		board.RestoreState()

		ply--

		if nullEval >= beta {
			return 0, beta
		}
	}

	staticEval := board.Evaluate()

	if !pv && !inCheck && depth <= 3 {
		margin := 120 * depth

		// reverse futility pruning
		if staticEval-margin >= beta {
			return 0, staticEval - margin
		}

		// futility pruning
		if staticEval+margin <= alpha {
			return 0, staticEval + margin
		}
	}

	// razoring
	if !pv && !inCheck && depth <= 3 {
		score := staticEval + 125

		if score < beta {
			if depth == 1 {
				return 0, max(score, quiesce(alpha, beta))
			}

			score += 175

			if score < beta && depth <= 2 {
				temp := quiesce(alpha, beta)
				if temp < beta {
					return 0, max(temp, score)
				}
			}
		}
	}

	originalAlpha := alpha
	bestScore := -board.LIMIT_SCORE
	bestMove := 0

	moves := board.MoveList{}
	board.GenerateAllMoves(&moves)
	scores := make([]int, moves.Count)

	for i := 0; i < moves.Count; i++ {
		if found && moves.Moves[i] == ttEntry.Move {
			// pv move
			scores[i] = 20000
			continue
		}

		scores[i] = scoreMove(moves.Moves[i], depth)
	}

	sortMoves(&moves, scores)
	legalMoves := 0

	for moveCount := 0; moveCount < moves.Count; moveCount++ {
		move := moves.Moves[moveCount]

		if legalMoves != 0 && depth <= 3 && legalMoves > 6+2*depth*depth && board.GetCapture(move) == 0 {
			continue
		}

		if !board.MakeMove(move) {
			continue
		}

		legalMoves++

		var score int

		if legalMoves == 1 {
			_, score = alphaBeta(-beta, -alpha, depth-1)
			score = -score
		} else {
			reduction := 0

			if depth < 3 || legalMoves <= 4 || inCheck {
				reduction = 0
			} else if board.GetPromotion(move) > 0 || board.GetCapture(move) > 0 {
				reduction = int(0.7 + 0.3*math.Log1p(float64(depth)) + 0.3*math.Log1p(float64(moveCount)))
			} else {
				reduction = int(1 + 0.5*math.Log1p(float64(depth)) + 0.7*math.Log1p(float64(moveCount)))
			}

			_, score = alphaBeta(-alpha-1, -alpha, depth-1-reduction)
			score = -score

			if score > alpha && score < beta {
				_, score = alphaBeta(-beta, -alpha, depth-1)
				score = -score
			}
		}

		board.RestoreState()

		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		if bestScore > alpha {
			alpha = bestScore
		}

		if alpha >= beta {
			if board.GetCapture(move) == 0 {
				historyHeuristic[board.Side][board.GetPiece(move)][board.GetTarget(move)] += depth * depth
			}

			killerHeuristic[board.Side][depth][1] = killerHeuristic[board.Side][depth][0]
			killerHeuristic[board.Side][depth][0] = move

			break
		}
	}

	// no legal moves found
	if legalMoves == 0 {
		// checkmate
		if inCheck {
			return 0, -board.MATE + ply
		}
		// stalemate
		return 0, 0
	}

	// update tt
	nodeType := board.AllNode
	if bestScore <= originalAlpha {
		nodeType = board.AllNode
	} else if bestScore >= beta {
		nodeType = board.CutNode
	} else {
		nodeType = board.PVNode
	}

	board.AddTTEntry(bestMove, bestScore, depth, nodeType)

	return bestMove, bestScore
}
