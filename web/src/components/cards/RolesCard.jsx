import React, { useState, useCallback } from 'react';
import { useApp } from '../../context/AppContext';
import { Shield, RefreshCw, ScrollText } from 'lucide-react';

export default function RolesCard() {
  const { isAuthenticated, makeAuthenticatedRequest } = useApp();
  const [result, setResult] = useState('Authenticate to view roles...');
  const [resultClass, setResultClass] = useState('');

  const getRoles = useCallback(async () => {
    if (!isAuthenticated) return;
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/roles');
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

  const getPermissions = useCallback(async () => {
    if (!isAuthenticated) return;
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/permissions');
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
    <section className="card roles-card">
      <div className="card-header">
        <div className="card-title">
          <span className="card-icon"><Shield size={18} /></span>
          <h2>Roles & Permissions</h2>
        </div>
        <div className="card-actions">
          <button 
            className="icon-btn auth-required" 
            onClick={getRoles}
            title="Load Roles" 
            disabled={!isAuthenticated}
          >
            <RefreshCw size={16} />
          </button>
          <button 
            className="icon-btn auth-required" 
            onClick={getPermissions}
            title="Load Permissions" 
            disabled={!isAuthenticated}
          >
            <ScrollText size={16} />
          </button>
        </div>
      </div>
      <div className="card-body">
        <div className="roles-result">
          <pre className={`code-block ${resultClass}`} id="rolesResult">{result}</pre>
        </div>
      </div>
    </section>
  );
}
