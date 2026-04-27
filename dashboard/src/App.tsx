import { useEffect, useState } from 'react'
import axios from 'axios'

const API = import.meta.env.VITE_API_URL || 'http://localhost:8080'

interface Review {
  id: number
  repo: string
  pr_number: number
  pr_title: string
  filename: string
  language: string
  review_text: string
  created_at: string
}

function App() {
  const [reviews, setReviews] = useState<Review[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Review | null>(null)

  useEffect(() => {
    axios.get(`${API}/reviews`)
      .then(res => setReviews(res.data))
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [])

  const grouped = reviews.reduce((acc, r) => {
    const key = `${r.repo}#${r.pr_number}`
    if (!acc[key]) acc[key] = []
    acc[key].push(r)
    return acc
  }, {} as Record<string, Review[]>)

  return (
    <div style={{ fontFamily: 'system-ui', maxWidth: 900, margin: '0 auto', padding: '2rem' }}>
      <h1 style={{ fontSize: 24, fontWeight: 600, marginBottom: 8 }}>
        AI Code Reviewer
      </h1>
      <p style={{ color: '#666', marginBottom: 32 }}>
        Automated code reviews powered by Groq + LLaMA
      </p>

      {loading && <p>Loading reviews...</p>}

      {!loading && Object.keys(grouped).length === 0 && (
        <div style={{ background: '#f5f5f5', padding: '2rem', borderRadius: 8, textAlign: 'center' }}>
          <p style={{ color: '#666' }}>No reviews yet. Open a pull request to trigger a review.</p>
        </div>
      )}

      {Object.entries(grouped).map(([key, files]) => {
        const first = files[0]
        return (
          <div key={key} style={{
            border: '1px solid #e5e7eb',
            borderRadius: 8,
            marginBottom: 16,
            overflow: 'hidden'
          }}>
            <div style={{
              background: '#f9fafb',
              padding: '1rem 1.25rem',
              borderBottom: '1px solid #e5e7eb'
            }}>
              <div style={{ fontWeight: 500, marginBottom: 4 }}>
                {first.pr_title}
              </div>
              <div style={{ fontSize: 13, color: '#6b7280' }}>
                {first.repo} · PR #{first.pr_number} · {files.length} file{files.length > 1 ? 's' : ''} reviewed · {new Date(first.created_at).toLocaleDateString()}
              </div>
            </div>

            {files.map(file => (
              <div
                key={file.id}
                onClick={() => setSelected(selected?.id === file.id ? null : file)}
                style={{
                  padding: '0.75rem 1.25rem',
                  borderBottom: '1px solid #f3f4f6',
                  cursor: 'pointer',
                  background: selected?.id === file.id ? '#eff6ff' : 'white'
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <code style={{ fontSize: 13 }}>{file.filename}</code>
                  <span style={{
                    fontSize: 11,
                    padding: '2px 8px',
                    borderRadius: 20,
                    background: '#e0f2fe',
                    color: '#0369a1'
                  }}>
                    {file.language}
                  </span>
                </div>

                {selected?.id === file.id && (
                  <div style={{
                    marginTop: 12,
                    padding: '1rem',
                    background: '#f8fafc',
                    borderRadius: 6,
                    fontSize: 13,
                    lineHeight: 1.7,
                    whiteSpace: 'pre-wrap',
                    color: '#374151'
                  }}>
                    {file.review_text}
                  </div>
                )}
              </div>
            ))}
          </div>
        )
      })}
    </div>
  )
}

export default App