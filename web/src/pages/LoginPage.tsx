import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { Card } from "../components/Card";
import { Input } from "../components/Input";
import { Button } from "../components/Button";

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
      navigate("/admin");
    } catch {
      setError("Invalid username or password");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-surface-secondary flex items-center justify-center px-4">
      <div className="w-full max-w-sm animate-fade-up">
        <Card padding="none" className="p-10">
          <div className="text-center mb-8">
            <h1 className="text-xl font-semibold tracking-tight text-text-primary">
              SodaTracker
            </h1>
            <p className="text-sm text-text-tertiary mt-1">Sign in to admin</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
            <Input
              label="Username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
            />
            <Input
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={error}
            />

            <Button type="submit" loading={loading} className="w-full">
              Sign In
            </Button>
          </form>

          <div className="mt-6 pt-6 border-t border-border-primary text-center">
            <button
              type="button"
              disabled={loading}
              onClick={() => {
                setUsername("admin");
                setPassword("dev-admin-password");
              }}
              className="text-xs text-text-muted hover:text-text-tertiary transition-colors duration-150 disabled:opacity-50"
            >
              Fill dev credentials
            </button>
          </div>
        </Card>
      </div>
    </div>
  );
}
