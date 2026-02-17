import React, { useState, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { Users, RefreshCw } from 'lucide-react';

export default function UsersCard() {
  const { isAuthenticated, makeAuthenticatedRequest } = useApp();
  const [result, setResult] = useState('Authenticate to manage users...');
  const [resultClass, setResultClass] = useState('');

  const getUsers = useCallback(async () => {
    if (!isAuthenticated) return;
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/users');
      const data = await response.json();
      
      if (data.success) {
        setResult(JSON.stringify(data, null, 2));
        setResultClass('success');
      } else {
        setResult('Error: ' + (data.error || data.message));
        setResultClass('error');
      }
    } catch (error) {
      setResult('Error: ' + error.message);
      setResultClass('error');
    }
  }, [isAuthenticated, makeAuthenticatedRequest]);

  return (
    <section className="card users-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Users size={18} /></span>
          <h2>Users</h2>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getUsers}
            title="Load Users" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        <div className="users-result">
          <pre className={`code-block ${resultClass}`} id="usersResult">{result}</pre>
        </div>
      </div>
    </section>
  );
}
